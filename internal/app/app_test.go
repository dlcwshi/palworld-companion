package app

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/dlcwshi/palworld-companion/internal/config"
	"github.com/dlcwshi/palworld-companion/internal/httpapi"
)

func TestNewFailsWhenDatabaseCannotInitialize(t *testing.T) {
	parent := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(parent, []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{App: config.AppConfig{MockMode: true}, Database: config.DatabaseConfig{Path: filepath.Join(parent, "companion.db")}}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	if _, err := New(cfg, httpapi.BuildInfo{}, logger); err == nil {
		t.Fatal("expected database initialization error")
	}
}
