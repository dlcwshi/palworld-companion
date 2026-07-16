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
func createSchemaFourDatabase(t *testing.T, path string) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`PRAGMA foreign_keys=OFF; CREATE TABLE schema_migrations(version INTEGER PRIMARY KEY,applied_at TEXT NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	for _, item := range migrations[:4] {
		if _, err = db.Exec(item.sql); err != nil {
			t.Fatalf("migration %d: %v", item.version, err)
		}
		if _, err = db.Exec(`INSERT INTO schema_migrations(version,applied_at) VALUES(?,'2026-01-01T00:00:00Z')`, item.version); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = db.Exec(`PRAGMA foreign_keys=ON`); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestUpgradeSchemaFourToFivePreservesDataAndBackfillsRoster(t *testing.T) {
	path := filepath.Join(t.TempDir(), "v4.db")
	db := createSchemaFourDatabase(t, path)
	now := "2026-07-15T14:00:00Z"
	adminResult, err := db.Exec(`
INSERT INTO users(username,display_name,password_hash,role,status,created_at,updated_at,approved_at)
VALUES('Owner','Owner','hash','admin','active',?,?,?)
`, now, now, now)
	if err != nil {
		t.Fatal(err)
	}
	adminID, _ := adminResult.LastInsertId()
	playerResult, err := db.Exec(`
INSERT INTO users(password_hash,steam_id,palworld_user_id,palworld_player_id,character_name,account_name,role,status,created_at,updated_at,last_seen_at)
VALUES('hash','76561198000000001','steam_76561198000000001',NULL,'Player','account','player','active',?,?,?)
`, now, now, "2026-07-15T14:30:00Z")
	if err != nil {
		t.Fatal(err)
	}
	playerID, _ := playerResult.LastInsertId()
	if _, err = db.Exec(`INSERT INTO sessions(user_id,token_hash,created_at,expires_at,last_seen_at) VALUES(?,'session',?,'2099-01-01T00:00:00Z',?)`, playerID, now, now); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`INSERT INTO tasks(title,created_at,updated_at,owner_id,created_by,visibility) VALUES('kept',?,?,?,?, 'personal')`, now, now, playerID, adminID); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`UPDATE system_settings SET value='true' WHERE key='setup_completed'`); err != nil {
		t.Fatal(err)
	}
	_ = db.Close()

	store, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	var users, sessions, tasks, rosterRows int
	for query, target := range map[string]*int{
		`SELECT count(*) FROM users`:         &users,
		`SELECT count(*) FROM sessions`:      &sessions,
		`SELECT count(*) FROM tasks`:         &tasks,
		`SELECT count(*) FROM player_roster`: &rosterRows,
	} {
		if err := store.SQL().QueryRow(query).Scan(target); err != nil {
			t.Fatal(err)
		}
	}
	if users != 2 || sessions != 1 || tasks != 1 || rosterRows != 1 {
		t.Fatalf("users=%d sessions=%d tasks=%d roster=%d", users, sessions, tasks, rosterRows)
	}
	var userID, character, firstSeen, lastOnline string
	var rosterPlayerID sql.NullString
	var online, level int
	if err := store.SQL().QueryRow(`
SELECT palworld_user_id,palworld_player_id,character_name,level,is_online,first_seen_at,last_online_at
FROM player_roster
`).Scan(&userID, &rosterPlayerID, &character, &level, &online, &firstSeen, &lastOnline); err != nil {
		t.Fatal(err)
	}
	if userID != "steam_76561198000000001" || rosterPlayerID.Valid || character != "Player" || level != 0 || online != 1 || firstSeen != now || lastOnline != "2026-07-15T14:30:00Z" {
		t.Fatalf("roster=%q %v %q %d %d %q %q", userID, rosterPlayerID, character, level, online, firstSeen, lastOnline)
	}
	var setup string
	if err := store.SQL().QueryRow(`SELECT value FROM system_settings WHERE key='setup_completed'`).Scan(&setup); err != nil || setup != "true" {
		t.Fatalf("setup=%q err=%v", setup, err)
	}
	var lastSuccess int
	if err := store.SQL().QueryRow(`SELECT count(*) FROM system_settings WHERE key='player_roster_last_success_at'`).Scan(&lastSuccess); err != nil || lastSuccess != 0 {
		t.Fatalf("last success rows=%d err=%v", lastSuccess, err)
	}
	rows, err := store.SQL().Query(`PRAGMA foreign_key_check`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	if rows.Next() {
		t.Fatal("foreign key violation")
	}
}

func TestUpgradeSchemaFiveDuplicateIdentityFailsWithoutPartialMigration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "duplicate-v4.db")
	db := createSchemaFourDatabase(t, path)
	if _, err := db.Exec(`
DROP INDEX idx_users_palworld_user_id;
DROP INDEX idx_users_palworld_player_id;
INSERT INTO users(password_hash,steam_id,palworld_user_id,palworld_player_id,character_name,role,status,created_at,updated_at)
VALUES
('hash','1','steam_1','same-player','One','player','active','2026-01-01T00:00:00Z','2026-01-01T00:00:00Z'),
('hash','2','steam_1','same-player','Two','player','active','2026-01-01T00:00:00Z','2026-01-01T00:00:00Z')
`); err != nil {
		t.Fatal(err)
	}
	_ = db.Close()

	if _, err := Open(path); err == nil {
		t.Fatal("expected duplicate roster identity migration failure")
	}
	raw, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer raw.Close()
	var version int
	if err := raw.QueryRow(`SELECT max(version) FROM schema_migrations`).Scan(&version); err != nil || version != 4 {
		t.Fatalf("version=%d err=%v", version, err)
	}
	var tableCount int
	if err := raw.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type='table' AND name='player_roster'`).Scan(&tableCount); err != nil || tableCount != 0 {
		t.Fatalf("player_roster table=%d err=%v", tableCount, err)
	}
}
