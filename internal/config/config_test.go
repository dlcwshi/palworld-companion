package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	content := "server:\n  listen: 127.0.0.1:9000\npalworld:\n  timeout: 2s\ncache:\n  players_ttl: 4s\napp:\n  mock_mode: true\n"
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Server.Listen != "127.0.0.1:9000" || cfg.Palworld.Timeout != 2*time.Second || cfg.Cache.PlayersTTL != 4*time.Second || !cfg.App.MockMode {
		t.Fatalf("unexpected config: %+v", cfg)
	}
	if cfg.Cache.InfoTTL != 30*time.Second {
		t.Fatalf("default info ttl = %s", cfg.Cache.InfoTTL)
	}
}

func TestLoadMissing(t *testing.T) {
	if _, err := Load(filepath.Join(t.TempDir(), "missing.yaml")); err == nil {
		t.Fatal("expected missing config error")
	}
}
