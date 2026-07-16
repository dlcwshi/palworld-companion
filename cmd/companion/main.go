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
	"strings"
	"syscall"
	"time"

	"github.com/dlcwshi/palworld-companion/internal/app"
	"github.com/dlcwshi/palworld-companion/internal/auth"
	"github.com/dlcwshi/palworld-companion/internal/config"
	"github.com/dlcwshi/palworld-companion/internal/httpapi"
	"github.com/dlcwshi/palworld-companion/internal/storage"
	"golang.org/x/term"
)

var (
	version   = "0.4.2-dev"
	commit    = ""
	buildTime = ""
)

func main() {
	if len(os.Args) > 1 {
		var err error
		switch os.Args[1] {
		case "users":
			err = runUsers(os.Args[2:])
		case "setup":
			err = runSetup(os.Args[2:])
		}
		if os.Args[1] == "users" || os.Args[1] == "setup" {
			if err != nil {
				slog.Error(os.Args[1]+" command failed", "error", err)
				os.Exit(1)
			}
			return
		}
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

func openAuth(configPath string) (*storage.DB, *auth.Service, error) {
	cfg, err := config.Load(configPath)
	if err != nil {
		return nil, nil, err
	}
	db, err := storage.Open(cfg.Database.Path)
	if err != nil {
		return nil, nil, err
	}
	return db, auth.NewService(auth.NewRepository(db.SQL()), nil, cfg.Auth.SessionTTL), nil
}
func runSetup(args []string) error {
	if len(args) == 0 || args[0] != "status" {
		return fmt.Errorf("usage: setup status --config <path>")
	}
	set := flag.NewFlagSet("setup status", flag.ContinueOnError)
	configPath := set.String("config", "deploy/config.example.yaml", "path to YAML configuration")
	if err := set.Parse(args[1:]); err != nil {
		return err
	}
	db, service, err := openAuth(*configPath)
	if err != nil {
		return err
	}
	defer db.Close()
	required, err := service.SetupRequired(context.Background())
	if err != nil {
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(map[string]bool{"setupRequired": required})
}

func readPasswordPair() (string, error) {
	fd := int(os.Stdin.Fd())
	if !term.IsTerminal(fd) {
		return "", fmt.Errorf("password input requires an interactive TTY")
	}
	_, _ = fmt.Fprint(os.Stderr, "Password: ")
	first, err := term.ReadPassword(fd)
	_, _ = fmt.Fprintln(os.Stderr)
	if err != nil {
		return "", err
	}
	_, _ = fmt.Fprint(os.Stderr, "Confirm password: ")
	second, err := term.ReadPassword(fd)
	_, _ = fmt.Fprintln(os.Stderr)
	if err != nil {
		return "", err
	}
	if string(first) != string(second) {
		return "", fmt.Errorf("password confirmation does not match")
	}
	if err := auth.ValidatePassword(string(first)); err != nil {
		return "", err
	}
	return string(first), nil
}

func runUsers(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: users list|set-role|create-admin|approve|reject|reset-password")
	}
	command := args[0]
	set := flag.NewFlagSet("users "+command, flag.ContinueOnError)
	configPath := set.String("config", "deploy/config.example.yaml", "path to YAML configuration")
	steamID := set.String("steam-id", "", "SteamID64")
	username := set.String("username", "", "administrator username")
	displayName := set.String("display-name", "", "optional display name")
	role := set.String("role", "", "admin or player")
	status := set.String("status", "", "optional account status filter")
	reason := set.String("reason", "", "optional rejection reason")
	if err := set.Parse(args[1:]); err != nil {
		return err
	}
	db, service, err := openAuth(*configPath)
	if err != nil {
		return err
	}
	defer db.Close()
	ctx := context.Background()
	switch command {
	case "list":
		users, err := service.ListUsers(ctx, *status)
		if err != nil {
			return err
		}
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(users)
	case "set-role":
		if *steamID == "" || (*role != auth.RoleAdmin && *role != auth.RolePlayer) {
			return fmt.Errorf("--steam-id and --role admin|player are required")
		}
		return service.SetRoleBySteamID(ctx, *steamID, *role)
	case "create-admin":
		if strings.TrimSpace(*username) == "" {
			return fmt.Errorf("--username is required")
		}
		password, err := readPasswordPair()
		if err != nil {
			return err
		}
		_, err = service.CreateRecoveryAdmin(ctx, *username, *displayName, password)
		return err
	case "approve":
		if *steamID == "" {
			return fmt.Errorf("--steam-id is required")
		}
		return service.ApproveBySteamID(ctx, *steamID)
	case "reject":
		if *steamID == "" {
			return fmt.Errorf("--steam-id is required")
		}
		return service.RejectBySteamID(ctx, *steamID, *reason)
	case "reset-password":
		if (*steamID == "") == (*username == "") {
			return fmt.Errorf("exactly one of --steam-id or --username is required")
		}
		password, err := readPasswordPair()
		if err != nil {
			return err
		}
		return service.ResetPasswordByIdentifier(ctx, *steamID, *username, password)
	default:
		return fmt.Errorf("unknown users command %q", command)
	}
}
