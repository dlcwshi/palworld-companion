package main

import (
	"context"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/dlcwshi/palworld-companion/internal/app"
	"github.com/dlcwshi/palworld-companion/internal/config"
	"github.com/dlcwshi/palworld-companion/internal/httpapi"
)

var (
	version   = "0.2.0-dev"
	commit    = ""
	buildTime = ""
)

func main() {
	configPath := flag.String("config", "deploy/config.example.yaml", "path to YAML configuration")
	flag.Parse()
	cfg, err := config.Load(*configPath)
	if err != nil {
		slog.Error("configuration failed", "error", err)
		os.Exit(1)
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: app.LogLevel(cfg.Logging.Level)}))
	build := httpapi.BuildInfo{Name: "Palworld Companion", Version: version, Commit: commit, BuildTime: buildTime}
	handler, err := app.New(cfg, build, logger)
	if err != nil {
		logger.Error("application setup failed", "error", err)
		os.Exit(1)
	}
	defer func() {
		if err := handler.Close(); err != nil {
			logger.Error("database close failed", "error", err)
		}
	}()
	server := &http.Server{Addr: cfg.Server.Listen, Handler: handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 10 * time.Second, IdleTimeout: 60 * time.Second}
	go func() {
		logger.Info("Palworld Companion listening", "address", cfg.Server.Listen, "version", version)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("HTTP server stopped unexpectedly", "error", err)
			os.Exit(1)
		}
	}()
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		logger.Error("graceful shutdown failed", "error", err)
	}
}
