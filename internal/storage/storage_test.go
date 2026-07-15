package storage

import (
	"context"
	"database/sql"
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

func TestUpgradeVersionOnePreservesTasksAsShared(t *testing.T) {
	path := filepath.Join(t.TempDir(), "v1.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	schema := `CREATE TABLE schema_migrations(version INTEGER PRIMARY KEY,applied_at TEXT NOT NULL);INSERT INTO schema_migrations VALUES(1,'2026-01-01T00:00:00Z');CREATE TABLE tasks(id INTEGER PRIMARY KEY AUTOINCREMENT,title TEXT NOT NULL,notes TEXT NOT NULL DEFAULT '',status TEXT NOT NULL DEFAULT 'pending',sort_order INTEGER NOT NULL DEFAULT 0,source_type TEXT NOT NULL DEFAULT 'manual',source_id INTEGER,created_at TEXT NOT NULL,updated_at TEXT NOT NULL,completed_at TEXT);INSERT INTO tasks(title,created_at,updated_at) VALUES('legacy','2026-01-01T00:00:00Z','2026-01-01T00:00:00Z');`
	if _, err := db.Exec(schema); err != nil {
		t.Fatal(err)
	}
	_ = db.Close()
	store, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	var visibility string
	var owner, creator sql.NullInt64
	if err := store.SQL().QueryRow(`SELECT visibility,owner_id,created_by FROM tasks WHERE title='legacy'`).Scan(&visibility, &owner, &creator); err != nil {
		t.Fatal(err)
	}
	if visibility != "shared" || owner.Valid || creator.Valid {
		t.Fatalf("visibility=%s owner=%v creator=%v", visibility, owner, creator)
	}
	var version int
	_ = store.SQL().QueryRow(`SELECT max(version) FROM schema_migrations`).Scan(&version)
	if version != CurrentSchemaVersion {
		t.Fatalf("version=%d", version)
	}
}

func TestRejectsFutureSchemaVersion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "future.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TABLE schema_migrations(version INTEGER PRIMARY KEY,applied_at TEXT NOT NULL);INSERT INTO schema_migrations VALUES(999,'now')`); err != nil {
		t.Fatal(err)
	}
	_ = db.Close()
	if _, err := Open(path); err == nil {
		t.Fatal("expected future schema rejection")
	}
}
