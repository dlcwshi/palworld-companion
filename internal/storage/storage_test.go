package storage

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestOpenCreatesDatabaseAndRunsMigrationOnce(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "companion.db")
	store, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	var versions int
	if err := store.SQL().QueryRow(`SELECT count(*) FROM schema_migrations`).Scan(&versions); err != nil {
		t.Fatal(err)
	}
	if versions != CurrentSchemaVersion {
		t.Fatalf("versions=%d", versions)
	}
	var version int
	if err := store.SQL().QueryRow(`SELECT max(version) FROM schema_migrations`).Scan(&version); err != nil {
		t.Fatal(err)
	}
	if version != CurrentSchemaVersion {
		t.Fatalf("version=%d", version)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	store, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.SQL().QueryRow(`SELECT count(*) FROM schema_migrations`).Scan(&versions); err != nil {
		t.Fatal(err)
	}
	if versions != CurrentSchemaVersion {
		t.Fatalf("migration repeated: versions=%d", versions)
	}
	if err := store.Ping(context.Background()); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(path); err != nil || info.IsDir() {
		t.Fatalf("database file: %v %v", info, err)
	}
}

func TestOpenFailsWhenParentIsFile(t *testing.T) {
	parent := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(parent, []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(filepath.Join(parent, "companion.db")); err == nil {
		t.Fatal("expected initialization failure")
	}
}
