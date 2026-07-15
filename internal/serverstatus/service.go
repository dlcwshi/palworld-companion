package serverstatus

import (
	"context"
	"time"

	"github.com/dlcwshi/palworld-companion/internal/palworld"
)

const publicUpstreamError = "Palworld API unavailable"

type Service struct {
	client  palworld.Client
	info    *Cache[palworld.Info]
	metrics *Cache[palworld.Metrics]
	players *Cache[palworld.Players]
}

func New(client palworld.Client, infoTTL, metricsTTL, playersTTL time.Duration) *Service {
	return &Service{client: client, info: NewCache[palworld.Info](infoTTL), metrics: NewCache[palworld.Metrics](metricsTTL), players: NewCache[palworld.Players](playersTTL)}
}

type SummaryResponse struct {
	Available bool           `json:"available"`
	Cached    bool           `json:"cached"`
	Stale     bool           `json:"stale"`
	UpdatedAt *time.Time     `json:"updatedAt"`
	Server    *ServerSummary `json:"server"`
	Error     *string        `json:"error"`
}
type ServerSummary struct {
	Name          string   `json:"name"`
	Version       *string  `json:"version"`
	Description   *string  `json:"description"`
	FPS           *float64 `json:"fps"`
	OnlinePlayers *int     `json:"onlinePlayers"`
	MaxPlayers    *int     `json:"maxPlayers"`
	UptimeSeconds *int64   `json:"uptimeSeconds"`
	WorldDays     *int     `json:"worldDays"`
	BaseCount     *int     `json:"baseCount"`
}
type PlayersResponse struct {
	Available bool           `json:"available"`
	Cached    bool           `json:"cached"`
	Stale     bool           `json:"stale"`
	UpdatedAt *time.Time     `json:"updatedAt"`
	Players   []PublicPlayer `json:"players"`
	Error     *string        `json:"error"`
}
type PublicPlayer struct {
	Name     string          `json:"name"`
	Level    *int            `json:"level"`
	Ping     *float64        `json:"ping"`
	Position *PublicPosition `json:"position"`
}
type PublicPosition struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

func (s *Service) Summary(ctx context.Context) SummaryResponse {
	info, infoMeta, infoErr := s.info.Get(ctx, s.client.GetInfo)
	metrics, metricsMeta, metricsErr := s.metrics.Get(ctx, s.client.GetMetrics)
	available := infoMeta.HasValue && metricsMeta.HasValue
	response := SummaryResponse{Available: available, Cached: infoMeta.Cached || metricsMeta.Cached, Stale: infoMeta.Stale || metricsMeta.Stale, UpdatedAt: oldestTime(infoMeta, metricsMeta)}
	if available {
		response.Server = &ServerSummary{
			Name: info.ServerName, Version: info.Version, Description: info.Description,
			FPS: &metrics.ServerFPS, OnlinePlayers: &metrics.CurrentPlayers, MaxPlayers: &metrics.MaxPlayers,
			UptimeSeconds: &metrics.Uptime, WorldDays: &metrics.Days, BaseCount: &metrics.BaseCampCount,
		}
	}
	if infoErr != nil || metricsErr != nil {
		message := publicUpstreamError
		response.Error = &message
	}
	return response
}

func (s *Service) Players(ctx context.Context) PlayersResponse {
	raw, meta, err := s.players.Get(ctx, s.client.GetPlayers)
	response := PlayersResponse{Available: meta.HasValue, Cached: meta.Cached, Stale: meta.Stale, Players: make([]PublicPlayer, 0)}
	if meta.HasValue {
		updated := meta.UpdatedAt
		response.UpdatedAt = &updated
		response.Players = make([]PublicPlayer, 0, len(raw.Players))
		for _, player := range raw.Players {
			public := PublicPlayer{Name: player.Name, Level: player.Level, Ping: player.Ping}
			if player.LocationX != nil && player.LocationY != nil {
				public.Position = &PublicPosition{X: *player.LocationX, Y: *player.LocationY}
			}
			response.Players = append(response.Players, public)
		}
	}
	if err != nil {
		message := publicUpstreamError
		response.Error = &message
	}
	return response
}

func oldestTime(values ...CacheMeta) *time.Time {
	var oldest time.Time
	for _, value := range values {
		if !value.HasValue {
			continue
		}
		if oldest.IsZero() || value.UpdatedAt.Before(oldest) {
			oldest = value.UpdatedAt
		}
	}
	if oldest.IsZero() {
		return nil
	}
	return &oldest
}
