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
	"github.com/dlcwshi/palworld-companion/internal/storage"
	"github.com/dlcwshi/palworld-companion/internal/tasks"
	"github.com/dlcwshi/palworld-companion/web"
)

type Application struct {
	http.Handler
	database *storage.DB
}

func (a *Application) Close() error { return a.database.Close() }

func New(cfg config.Config, build httpapi.BuildInfo, logger *slog.Logger) (*Application, error) {
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
	database, err := storage.Open(cfg.Database.Path)
	if err != nil {
		return nil, fmt.Errorf("initialize database: %w", err)
	}
	taskService := tasks.NewService(tasks.NewRepository(database.SQL()))
	dist, err := fs.Sub(web.Assets, "dist")
	if err != nil {
		_ = database.Close()
		return nil, fmt.Errorf("open embedded frontend: %w", err)
	}
	return &Application{Handler: httpapi.New(status, taskService, build, logger, dist), database: database}, nil
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
