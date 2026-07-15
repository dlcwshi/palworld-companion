package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"time"

	"github.com/dlcwshi/palworld-companion/internal/palworld"
)

type Service struct {
	repo     *Repository
	players  palworld.Client
	verifier Verifier
	enabled  bool
	baseURL  string
	ttl      time.Duration
	admins   map[string]bool
	now      func() time.Time
}

func NewService(repo *Repository, players palworld.Client, verifier Verifier, enabled bool, baseURL string, ttl time.Duration, adminIDs []string) *Service {
	m := map[string]bool{}
	for _, id := range adminIDs {
		m[id] = true
	}
	return &Service{repo: repo, players: players, verifier: verifier, enabled: enabled, baseURL: baseURL, ttl: ttl, admins: m, now: time.Now}
}
func (s *Service) Enabled() bool             { return s.enabled }
func (s *Service) SessionTTL() time.Duration { return s.ttl }
func randomToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
func tokenHash(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}
func (s *Service) callbackURL(state string) string {
	return s.baseURL + "api/v1/auth/steam/callback?state=" + url.QueryEscape(state)
}

func (s *Service) Begin(ctx context.Context, returnTo string) (string, string, error) {
	if !s.enabled {
		return "", "", ErrAuthDisabled
	}
	safe, err := SafeReturnPath(returnTo)
	if err != nil {
		return "", "", err
	}
	state, err := randomToken()
	if err != nil {
		return "", "", err
	}
	now := s.now().UTC()
	if err := s.repo.CreateFlow(ctx, tokenHash(state), safe, now, now.Add(10*time.Minute)); err != nil {
		return "", "", err
	}
	q := url.Values{"openid.ns": {"http://specs.openid.net/auth/2.0"}, "openid.mode": {"checkid_setup"}, "openid.claimed_id": {"http://specs.openid.net/auth/2.0/identifier_select"}, "openid.identity": {"http://specs.openid.net/auth/2.0/identifier_select"}, "openid.return_to": {s.callbackURL(state)}, "openid.realm": {s.baseURL}}
	return SteamOpenIDEndpoint + "?" + q.Encode(), state, nil
}

func (s *Service) Callback(ctx context.Context, values url.Values, cookieState string) (User, string, string, error) {
	if !s.enabled {
		return User{}, "", "", ErrAuthDisabled
	}
	state := values.Get("state")
	if state == "" || cookieState == "" || state != cookieState {
		return User{}, "", "", ErrInvalidFlow
	}
	flow, err := s.repo.GetFlow(ctx, tokenHash(state))
	if err != nil || flow.ConsumedAt != nil || !flow.ExpiresAt.After(s.now()) {
		return User{}, "", "", ErrInvalidFlow
	}
	if values.Get("openid.mode") != "id_res" || values.Get("openid.return_to") != s.callbackURL(state) {
		return User{}, "", "", ErrInvalidFlow
	}
	steamID, err := s.verifier.Verify(ctx, values)
	if err != nil {
		return User{}, "", "", err
	}
	if err := s.repo.ConsumeFlow(ctx, flow.ID, s.now().UTC()); err != nil {
		return User{}, "", "", err
	}
	user, err := s.login(ctx, steamID)
	if err != nil {
		return User{}, "", "", err
	}
	token, err := randomToken()
	if err != nil {
		return User{}, "", "", err
	}
	now := s.now().UTC()
	if err := s.repo.CreateSession(ctx, user.ID, tokenHash(token), now, now.Add(s.ttl)); err != nil {
		return User{}, "", "", err
	}
	return user, token, flow.ReturnPath, nil
}

func (s *Service) login(ctx context.Context, steamID string) (User, error) {
	now := s.now().UTC()
	user, err := s.repo.FindBySteamID(ctx, steamID)
	if err == nil {
		if user.Status == StatusDisabled {
			return User{}, ErrAccountDisabled
		}
		if user.Status == StatusDeleted {
			return User{}, ErrAccountDeleted
		}
		if s.admins[steamID] {
			user.Role = RoleAdmin
		}
		fresh, e := palworld.GetPlayersFreshForIdentityBinding(ctx, s.players)
		updated := false
		if e == nil {
			for _, p := range fresh.Players {
				if p.UserID == "steam_"+steamID {
					user.CharacterName = p.Name
					user.AccountName = p.AccountName
					user.PalworldPlayerID = p.PlayerID
					updated = true
					break
				}
			}
		}
		if err := s.repo.UpdateLogin(ctx, user, now, updated); err != nil {
			return User{}, err
		}
		return s.repo.FindBySteamID(ctx, steamID)
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return User{}, err
	}
	fresh, err := palworld.GetPlayersFreshForIdentityBinding(ctx, s.players)
	if err != nil {
		return User{}, ErrUpstream
	}
	for _, p := range fresh.Players {
		if p.UserID == "steam_"+steamID {
			role := RolePlayer
			if s.admins[steamID] {
				role = RoleAdmin
			}
			seen := now
			created := User{SteamID: steamID, PalworldUserID: p.UserID, PalworldPlayerID: p.PlayerID, CharacterName: p.Name, AccountName: p.AccountName, Role: role, Status: StatusActive, CreatedAt: now, UpdatedAt: now, LastLoginAt: now, LastSeenAt: &seen}
			result, err := s.repo.CreateUser(ctx, created)
			if err != nil {
				return User{}, err
			}
			if result.Status != StatusActive {
				if result.Status == StatusDisabled {
					return User{}, ErrAccountDisabled
				}
				return User{}, ErrAccountDeleted
			}
			return result, nil
		}
	}
	return User{}, ErrPlayerOffline
}

func (s *Service) Authenticate(ctx context.Context, token string) (User, error) {
	if !s.enabled {
		return User{}, ErrAuthDisabled
	}
	if token == "" {
		return User{}, ErrUnauthenticated
	}
	return s.repo.Authenticate(ctx, tokenHash(token), s.now().UTC())
}
func (s *Service) Logout(ctx context.Context, token string) error {
	if token == "" {
		return nil
	}
	return s.repo.RevokeToken(ctx, tokenHash(token), s.now().UTC())
}
func (s *Service) ListUsers(ctx context.Context) ([]User, error) { return s.repo.ListUsers(ctx) }
func (s *Service) SetStatus(ctx context.Context, current, target int64, status string) error {
	if status != StatusActive && status != StatusDisabled && status != StatusDeleted {
		return fmt.Errorf("invalid status")
	}
	return s.repo.SetStatus(ctx, current, target, status, s.now().UTC())
}
func (s *Service) RevokeUserSessions(ctx context.Context, id int64) error {
	return s.repo.RevokeUserSessions(ctx, id, s.now().UTC())
}
func (s *Service) SetRoleBySteamID(ctx context.Context, steamID, role string) error {
	return s.repo.SetRoleBySteamID(ctx, steamID, role, s.now().UTC())
}
func (s *Service) Cleanup(ctx context.Context) { s.repo.Cleanup(ctx, s.now().UTC()) }
