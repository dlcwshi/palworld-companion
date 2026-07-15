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
func TestUpgradeSchemaThreePreservesUsersSessionsAndTasks(t *testing.T) {
	path := filepath.Join(t.TempDir(), "v3.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`PRAGMA foreign_keys=ON; CREATE TABLE schema_migrations(version INTEGER PRIMARY KEY,applied_at TEXT NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	for _, item := range migrations[:3] {
		if _, err = db.Exec(item.sql); err != nil {
			t.Fatalf("migration %d: %v", item.version, err)
		}
		if _, err = db.Exec(`INSERT INTO schema_migrations(version,applied_at) VALUES(?,'2026-01-01T00:00:00Z')`, item.version); err != nil {
			t.Fatal(err)
		}
	}
	now := "2026-01-01T00:00:00Z"
	result, err := db.Exec(`INSERT INTO users(steam_id,palworld_user_id,palworld_player_id,character_name,role,status,created_at,updated_at,last_login_at) VALUES('76561198000000000','steam_76561198000000000','player-1','Legacy','admin','active',?,?,?)`, now, now, now)
	if err != nil {
		t.Fatal(err)
	}
	userID, _ := result.LastInsertId()
	if _, err = db.Exec(`INSERT INTO sessions(user_id,token_hash,created_at,expires_at,last_seen_at) VALUES(?,'hash',?,'2099-01-01T00:00:00Z',?)`, userID, now, now); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`INSERT INTO tasks(title,created_at,updated_at,owner_id,created_by,visibility) VALUES('kept',?,?,?,?,'personal')`, now, now, userID, userID); err != nil {
		t.Fatal(err)
	}
	_ = db.Close()
	store, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	var steamID, status, taskTitle, setup string
	var password sql.NullString
	if err = store.SQL().QueryRow(`SELECT steam_id,status,password_hash FROM users WHERE id=?`, userID).Scan(&steamID, &status, &password); err != nil {
		t.Fatal(err)
	}
	if steamID != "76561198000000000" || status != "active" || password.Valid {
		t.Fatalf("steam=%s status=%s password=%v", steamID, status, password)
	}
	if err = store.SQL().QueryRow(`SELECT title FROM tasks WHERE owner_id=?`, userID).Scan(&taskTitle); err != nil || taskTitle != "kept" {
		t.Fatalf("task=%s err=%v", taskTitle, err)
	}
	var sessions int
	if err = store.SQL().QueryRow(`SELECT count(*) FROM sessions WHERE user_id=?`, userID).Scan(&sessions); err != nil || sessions != 1 {
		t.Fatalf("sessions=%d err=%v", sessions, err)
	}
	if err = store.SQL().QueryRow(`SELECT value FROM system_settings WHERE key='setup_completed'`).Scan(&setup); err != nil || setup != "true" {
		t.Fatalf("setup=%q err=%v", setup, err)
	}
	rows, err := store.SQL().Query(`PRAGMA foreign_key_check`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	if rows.Next() {
		t.Fatal("foreign key violation after migration")
	}
}
