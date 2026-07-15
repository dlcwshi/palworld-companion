package storage

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

const CurrentSchemaVersion = 3

type DB struct{ sql *sql.DB }
type migration struct {
	version int
	sql     string
}

var migrations = []migration{{version: 1, sql: `
CREATE TABLE tasks (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    title TEXT NOT NULL CHECK (length(trim(title)) > 0 AND length(title) <= 200),
    notes TEXT NOT NULL DEFAULT '' CHECK (length(notes) <= 4000),
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'completed')),
    sort_order INTEGER NOT NULL DEFAULT 0,
    source_type TEXT NOT NULL DEFAULT 'manual' CHECK (source_type IN ('manual', 'crafting_plan')),
    source_id INTEGER,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    completed_at TEXT
);
CREATE INDEX idx_tasks_list ON tasks(status, sort_order, created_at DESC);
`}, {version: 2, sql: `
CREATE TABLE users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    steam_id TEXT NOT NULL UNIQUE,
    palworld_user_id TEXT NOT NULL UNIQUE,
    palworld_player_id TEXT NOT NULL DEFAULT '',
    character_name TEXT NOT NULL,
    account_name TEXT NOT NULL DEFAULT '',
    role TEXT NOT NULL DEFAULT 'player' CHECK (role IN ('admin', 'player')),
    status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'disabled', 'deleted')),
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    last_login_at TEXT NOT NULL,
    last_seen_at TEXT,
    deleted_at TEXT
);
CREATE TABLE sessions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL REFERENCES users(id),
    token_hash TEXT NOT NULL UNIQUE,
    created_at TEXT NOT NULL,
    expires_at TEXT NOT NULL,
    last_seen_at TEXT NOT NULL,
    revoked_at TEXT
);
CREATE TABLE auth_flows (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    state_hash TEXT NOT NULL UNIQUE,
    return_path TEXT NOT NULL,
    created_at TEXT NOT NULL,
    expires_at TEXT NOT NULL,
    consumed_at TEXT
);
CREATE INDEX idx_sessions_user_id ON sessions(user_id);
CREATE INDEX idx_sessions_token_hash ON sessions(token_hash);
CREATE INDEX idx_users_steam_id ON users(steam_id);
CREATE INDEX idx_users_palworld_user_id ON users(palworld_user_id);
`}, {version: 3, sql: `
ALTER TABLE tasks ADD COLUMN owner_id INTEGER REFERENCES users(id);
ALTER TABLE tasks ADD COLUMN created_by INTEGER REFERENCES users(id);
ALTER TABLE tasks ADD COLUMN visibility TEXT NOT NULL DEFAULT 'shared' CHECK (visibility IN ('personal', 'shared'));
CREATE INDEX idx_tasks_owner_id ON tasks(owner_id);
CREATE INDEX idx_tasks_created_by ON tasks(created_by);
CREATE INDEX idx_tasks_visibility ON tasks(visibility);
CREATE INDEX idx_tasks_status ON tasks(status);
`}}

func Open(path string) (*DB, error) {
	if path == "" {
		return nil, fmt.Errorf("database path is required")
	}
	parent := filepath.Dir(path)
	if parent != "." {
		if err := os.MkdirAll(parent, 0750); err != nil {
			return nil, fmt.Errorf("create database directory: %w", err)
		}
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0)
	store := &DB{sql: db}
	if err := store.initialize(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	if path != ":memory:" {
		if err := os.Chmod(path, 0640); err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("set database permissions: %w", err)
		}
	}
	return store, nil
}

func (d *DB) SQL() *sql.DB                   { return d.sql }
func (d *DB) Ping(ctx context.Context) error { return d.sql.PingContext(ctx) }
func (d *DB) Close() error                   { return d.sql.Close() }

func (d *DB) initialize(ctx context.Context) error {
	for _, pragma := range []string{"PRAGMA foreign_keys = ON", "PRAGMA journal_mode = WAL", "PRAGMA busy_timeout = 5000"} {
		if _, err := d.sql.ExecContext(ctx, pragma); err != nil {
			return fmt.Errorf("configure database: %w", err)
		}
	}
	if _, err := d.sql.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (version INTEGER PRIMARY KEY, applied_at TEXT NOT NULL)`); err != nil {
		return fmt.Errorf("initialize migrations: %w", err)
	}
	var highest sql.NullInt64
	if err := d.sql.QueryRowContext(ctx, `SELECT max(version) FROM schema_migrations`).Scan(&highest); err != nil {
		return fmt.Errorf("read schema version: %w", err)
	}
	if highest.Valid && highest.Int64 > CurrentSchemaVersion {
		return fmt.Errorf("database schema version %d is newer than supported version %d", highest.Int64, CurrentSchemaVersion)
	}
	for _, item := range migrations {
		var exists int
		err := d.sql.QueryRowContext(ctx, `SELECT 1 FROM schema_migrations WHERE version = ?`, item.version).Scan(&exists)
		if err == nil {
			continue
		}
		if err != sql.ErrNoRows {
			return fmt.Errorf("check migration %d: %w", item.version, err)
		}
		tx, err := d.sql.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("begin migration %d: %w", item.version, err)
		}
		if _, err = tx.ExecContext(ctx, item.sql); err == nil {
			_, err = tx.ExecContext(ctx, `INSERT INTO schema_migrations(version, applied_at) VALUES (?, ?)`, item.version, time.Now().UTC().Format(time.RFC3339Nano))
		}
		if err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("apply migration %d: %w", item.version, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit migration %d: %w", item.version, err)
		}
	}
	return nil
}
