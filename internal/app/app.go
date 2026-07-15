package app

import (
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"strings"

	"github.com/dlcwshi/palworld-companion/internal/config"
	"github.com/dlcwshi/palworld-companion/internal/httpapi"
	"github.com/dlcwshi/palworld-companion/internal/palworld"
	"github.com/dlcwshi/palworld-companion/internal/serverstatus"
	"github.com/dlcwshi/palworld-companion/web"
)

func New(cfg config.Config, build httpapi.BuildInfo, logger *slog.Logger) (http.Handler, error) {
	var client palworld.Client
	if cfg.App.MockMode {
		client = palworld.MockClient{}
		logger.Info("mock mode enabled; Palworld API will not be contacted")
	} else {
		httpClient, err := palworld.NewHTTPClient(cfg.Palworld.BaseURL, cfg.Palworld.Username, cfg.Palworld.Password, cfg.Palworld.Timeout)
		if err != nil {
			return nil, err
		}
		client = httpClient
	}
	status := serverstatus.New(client, cfg.Cache.InfoTTL, cfg.Cache.MetricsTTL, cfg.Cache.PlayersTTL)
	dist, err := fs.Sub(web.Assets, "dist")
	if err != nil {
		return nil, fmt.Errorf("open embedded frontend: %w", err)
	}
	return httpapi.New(status, build, logger, dist), nil
}

func LogLevel(raw string) slog.Level {
	switch strings.ToLower(raw) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
