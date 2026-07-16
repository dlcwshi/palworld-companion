package serverstatus

import (
	"context"
	"time"

	"github.com/dlcwshi/palworld-companion/internal/palworld"
	"github.com/dlcwshi/palworld-companion/internal/roster"
)

const publicUpstreamError = "Palworld API unavailable"

type Service struct {
	client  palworld.Client
	roster  *roster.Service
	info    *Cache[palworld.Info]
	metrics *Cache[palworld.Metrics]
}

func New(client palworld.Client, playerRoster *roster.Service, infoTTL, metricsTTL time.Duration) *Service {
	return &Service{client: client, roster: playerRoster, info: NewCache[palworld.Info](infoTTL), metrics: NewCache[palworld.Metrics](metricsTTL)}
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
	Name               string   `json:"name"`
	Version            *string  `json:"version"`
	Description        *string  `json:"description"`
	FPS                *float64 `json:"fps"`
	OnlinePlayers      *int     `json:"onlinePlayers"`
	OnlinePlayersKnown bool     `json:"onlinePlayersKnown"`
	MaxPlayers         *int     `json:"maxPlayers"`
	UptimeSeconds      *int64   `json:"uptimeSeconds"`
	WorldDays          *int     `json:"worldDays"`
	BaseCount          *int     `json:"baseCount"`
}

func (s *Service) Summary(ctx context.Context) SummaryResponse {
	info, infoMeta, infoErr := s.info.Get(ctx, s.client.GetInfo)
	metrics, metricsMeta, metricsErr := s.metrics.Get(ctx, s.client.GetMetrics)
	presence := s.roster.Players(ctx)
	available := infoMeta.HasValue && metricsMeta.HasValue
	response := SummaryResponse{
		Available: available,
		Cached:    infoMeta.Cached || metricsMeta.Cached || presence.Cached,
		Stale:     infoMeta.Stale || metricsMeta.Stale || presence.Stale,
		UpdatedAt: oldestTime(infoMeta, metricsMeta),
	}
	if available {
		response.Server = &ServerSummary{
			Name: info.ServerName, Version: info.Version, Description: info.Description,
			FPS: &metrics.ServerFPS, OnlinePlayers: presence.Counts.CurrentOnline,
			OnlinePlayersKnown: presence.CurrentStatusKnown, MaxPlayers: &metrics.MaxPlayers,
			UptimeSeconds: &metrics.Uptime, WorldDays: &metrics.Days, BaseCount: &metrics.BaseCampCount,
		}
	}
	if infoErr != nil || metricsErr != nil || presence.Error != nil {
		message := publicUpstreamError
		response.Error = &message
	}
	return response
}

func (s *Service) Players(ctx context.Context) roster.Response {
	return s.roster.Players(ctx)
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
