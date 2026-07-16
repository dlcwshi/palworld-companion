package roster

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/dlcwshi/palworld-companion/internal/palworld"
)

const (
	PublicUpstreamError = "palworld_unavailable"
	PublicRosterError   = "roster_unavailable"
)

var ErrPersistence = errors.New("player roster persistence failed")

type cacheEntry struct {
	snapshot    palworld.Players
	completedAt time.Time
}

type Service struct {
	repo   *Repository
	client palworld.Client
	ttl    time.Duration
	now    func() time.Time

	mu          sync.Mutex
	cache       *cacheEntry
	failureAt   time.Time
	failureCode string
}

func NewService(repo *Repository, client palworld.Client, ttl time.Duration) *Service {
	return &Service{repo: repo, client: client, ttl: ttl, now: time.Now}
}

func (s *Service) Players(ctx context.Context) Response {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.now().UTC()
	if s.cache != nil && s.ttl > 0 && now.Sub(s.cache.completedAt) < s.ttl {
		return s.responseFromState(ctx, true, false, true, s.cache.snapshot, nil)
	}
	if !s.failureAt.IsZero() && s.ttl > 0 && now.Sub(s.failureAt) < s.ttl {
		code := s.failureCode
		return s.responseFromState(ctx, false, true, false, palworld.Players{}, &code)
	}
	snapshot, err := s.loadFresh(ctx)
	if err != nil {
		code := PublicUpstreamError
		if errors.Is(err, ErrPersistence) {
			code = PublicRosterError
		}
		s.failureAt, s.failureCode = now, code
		return s.responseFromState(ctx, false, true, false, palworld.Players{}, &code)
	}
	s.failureAt, s.failureCode = time.Time{}, ""
	return s.responseFromState(ctx, false, false, true, snapshot, nil)
}

func (s *Service) FreshPlayers(ctx context.Context) (palworld.Players, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	snapshot, err := s.loadFresh(ctx)
	if err == nil {
		s.failureAt, s.failureCode = time.Time{}, ""
	}
	return snapshot, err
}

func (s *Service) loadFresh(ctx context.Context) (palworld.Players, error) {
	if s.client == nil {
		return palworld.Players{}, errors.New("Palworld client unavailable")
	}
	snapshot, err := s.client.GetPlayers(ctx)
	if err != nil {
		return palworld.Players{}, err
	}
	if err := palworld.ValidatePlayers(snapshot); err != nil {
		return palworld.Players{}, err
	}
	completedAt := s.now().UTC()
	if err := s.repo.ApplySnapshot(ctx, snapshot, completedAt); err != nil {
		return palworld.Players{}, fmt.Errorf("%w: %v", ErrPersistence, err)
	}
	s.cache = &cacheEntry{snapshot: snapshot, completedAt: completedAt}
	return snapshot, nil
}

func (s *Service) responseFromState(ctx context.Context, cached, stale, known bool, live palworld.Players, publicError *string) Response {
	players, lastSuccess, err := s.repo.State(ctx)
	if err != nil {
		code := PublicRosterError
		return Response{
			Available: false, Cached: false, Stale: true, CurrentStatusKnown: false,
			Error: &code, Counts: Counts{}, Players: make([]PublicPlayer, 0),
		}
	}

	response := Response{
		Available: len(players) > 0 || lastSuccess != nil,
		Cached:    cached, Stale: stale, CurrentStatusKnown: known,
		UpdatedAt: lastSuccess, Error: publicError,
		Counts: Counts{Total: len(players)}, Players: make([]PublicPlayer, 0, len(players)),
	}
	liveByUserID := make(map[string]palworld.Player, len(live.Players))
	for _, player := range live.Players {
		liveByUserID[player.UserID] = player
	}
	currentOnline, currentOffline := 0, 0
	for _, player := range players {
		lastKnown := StatusOffline
		if player.IsOnline {
			lastKnown = StatusOnline
			response.Counts.LastKnownOnline++
		} else {
			response.Counts.LastKnownOffline++
		}
		status := StatusUnknown
		if known {
			status = lastKnown
			if player.IsOnline {
				currentOnline++
			} else {
				currentOffline++
			}
		}
		public := PublicPlayer{
			Name: player.CharacterName, Level: player.Level, Status: status,
			LastKnownStatus: lastKnown, LastOnlineAt: player.LastOnlineAt,
		}
		if known && player.IsOnline {
			if realtime, ok := liveByUserID[player.PalworldUserID]; ok {
				public.Ping = realtime.Ping
				if realtime.LocationX != nil && realtime.LocationY != nil {
					public.Position = &PublicPosition{X: *realtime.LocationX, Y: *realtime.LocationY, Z: realtime.LocationZ}
				}
			}
		}
		response.Players = append(response.Players, public)
	}
	if known {
		response.Counts.CurrentOnline = &currentOnline
		response.Counts.CurrentOffline = &currentOffline
	}
	sortPublicPlayers(response.Players, known)
	return response
}

func sortPublicPlayers(players []PublicPlayer, known bool) {
	sort.SliceStable(players, func(i, j int) bool {
		left, right := players[i], players[j]
		leftOnline := left.LastKnownStatus == StatusOnline
		rightOnline := right.LastKnownStatus == StatusOnline
		if known {
			leftOnline = left.Status == StatusOnline
			rightOnline = right.Status == StatusOnline
		}
		if leftOnline != rightOnline {
			return leftOnline
		}
		if !left.LastOnlineAt.Equal(right.LastOnlineAt) {
			return left.LastOnlineAt.After(right.LastOnlineAt)
		}
		leftName, rightName := strings.ToLower(left.Name), strings.ToLower(right.Name)
		if leftName != rightName {
			return leftName < rightName
		}
		return left.Name < right.Name
	})
}
