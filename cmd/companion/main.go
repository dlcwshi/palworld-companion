package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/dlcwshi/palworld-companion/internal/app"
	"github.com/dlcwshi/palworld-companion/internal/auth"
	"github.com/dlcwshi/palworld-companion/internal/config"
	"github.com/dlcwshi/palworld-companion/internal/httpapi"
	"github.com/dlcwshi/palworld-companion/internal/storage"
)

var (
	version   = "0.2.0-dev"
	commit    = ""
	buildTime = ""
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "users" {
		if err := runUsers(os.Args[2:]); err != nil {
			slog.Error("users command failed", "error", err)
			os.Exit(1)
		}
		return
	}
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

func runUsers(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: users list|set-role")
	}
	command := args[0]
	set := flag.NewFlagSet("users "+command, flag.ContinueOnError)
	configPath := set.String("config", "deploy/config.example.yaml", "path to YAML configuration")
	steamID := set.String("steam-id", "", "SteamID64")
	role := set.String("role", "", "admin or player")
	if err := set.Parse(args[1:]); err != nil {
		return err
	}
	cfg, err := config.Load(*configPath)
	if err != nil {
		return err
	}
	db, err := storage.Open(cfg.Database.Path)
	if err != nil {
		return err
	}
	defer db.Close()
	repo := auth.NewRepository(db.SQL())
	switch command {
	case "list":
		users, err := repo.ListUsers(context.Background())
		if err != nil {
			return err
		}
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(users)
	case "set-role":
		if *steamID == "" {
			return fmt.Errorf("--steam-id is required")
		}
		return repo.SetRoleBySteamID(context.Background(), *steamID, *role, time.Now().UTC())
	default:
		return fmt.Errorf("unknown users command %q", command)
	}
}
