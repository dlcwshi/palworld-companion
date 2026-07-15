package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

type Repository struct{ db *sql.DB }

type rowScanner interface{ Scan(...any) error }
type dbRunner interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func NewRepository(db *sql.DB) *Repository { return &Repository{db: db} }
func stamp(t time.Time) string             { return t.UTC().Format(time.RFC3339Nano) }

const userColumns = `id,username,display_name,password_hash,steam_id,palworld_user_id,palworld_player_id,character_name,account_name,role,status,previous_status,created_at,updated_at,last_login_at,last_seen_at,deleted_at,approved_at,approved_by,rejected_at,rejected_by,rejection_reason`

func nullableString(value sql.NullString) *string {
	if !value.Valid {
		return nil
	}
	v := value.String
	return &v
}
func nullableTime(value sql.NullString) *time.Time {
	if !value.Valid {
		return nil
	}
	v, err := time.Parse(time.RFC3339Nano, value.String)
	if err != nil {
		return nil
	}
	return &v
}
func nullableInt64(value sql.NullInt64) *int64 {
	if !value.Valid {
		return nil
	}
	v := value.Int64
	return &v
}
func scanUser(row rowScanner) (User, error) {
	var u User
	var username, passwordHash, steamID, palworldUserID, palworldPlayerID, previous sql.NullString
	var created, updated string
	var lastLogin, lastSeen, deleted, approved, rejected sql.NullString
	var approvedBy, rejectedBy sql.NullInt64
	if err := row.Scan(&u.ID, &username, &u.DisplayName, &passwordHash, &steamID, &palworldUserID, &palworldPlayerID, &u.CharacterName, &u.AccountName, &u.Role, &u.Status, &previous, &created, &updated, &lastLogin, &lastSeen, &deleted, &approved, &approvedBy, &rejected, &rejectedBy, &u.RejectionReason); err != nil {
		return User{}, err
	}
	u.Username = nullableString(username)
	u.PasswordHash = nullableString(passwordHash)
	u.SteamID = nullableString(steamID)
	u.PalworldUserID = nullableString(palworldUserID)
	u.PalworldPlayerID = nullableString(palworldPlayerID)
	u.PreviousStatus = nullableString(previous)
	u.CreatedAt, _ = time.Parse(time.RFC3339Nano, created)
	u.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updated)
	u.LastLoginAt = nullableTime(lastLogin)
	u.LastSeenAt = nullableTime(lastSeen)
	u.DeletedAt = nullableTime(deleted)
	u.ApprovedAt = nullableTime(approved)
	u.ApprovedBy = nullableInt64(approvedBy)
	u.RejectedAt = nullableTime(rejected)
	u.RejectedBy = nullableInt64(rejectedBy)
	return u, nil
}

func (r *Repository) SetupRequired(ctx context.Context) (bool, error) {
	var value string
	if err := r.db.QueryRowContext(ctx, `SELECT value FROM system_settings WHERE key='setup_completed'`).Scan(&value); err != nil {
		return false, err
	}
	return value != "true", nil
}

func (r *Repository) CreateInitialAdmin(ctx context.Context, username, displayName, passwordHash, sessionHash string, now, expires time.Time) (User, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return User{}, err
	}
	defer tx.Rollback()
	var completed string
	if err := tx.QueryRowContext(ctx, `SELECT value FROM system_settings WHERE key='setup_completed'`).Scan(&completed); err != nil {
		return User{}, err
	}
	if completed == "true" {
		return User{}, ErrAlreadyInitialized
	}
	result, err := tx.ExecContext(ctx, `INSERT INTO users(username,display_name,password_hash,role,status,created_at,updated_at,last_login_at,approved_at) VALUES(?,?,?,'admin','active',?,?,?,?)`, username, displayName, passwordHash, stamp(now), stamp(now), stamp(now), stamp(now))
	if err != nil {
		return User{}, classifyConstraint(err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return User{}, err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO sessions(user_id,token_hash,created_at,expires_at,last_seen_at) VALUES(?,?,?,?,?)`, id, sessionHash, stamp(now), stamp(expires), stamp(now)); err != nil {
		return User{}, err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE system_settings SET value='true',updated_at=? WHERE key='setup_completed' AND value='false'`, stamp(now)); err != nil {
		return User{}, err
	}
	u, err := scanUser(tx.QueryRowContext(ctx, `SELECT `+userColumns+` FROM users WHERE id=?`, id))
	if err != nil {
		return User{}, err
	}
	if err := tx.Commit(); err != nil {
		return User{}, err
	}
	return u, nil
}

func (r *Repository) CreateRecoveryAdmin(ctx context.Context, username, displayName, passwordHash string, now time.Time) (User, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return User{}, err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `INSERT INTO users(username,display_name,password_hash,role,status,created_at,updated_at,approved_at) VALUES(?,?,?,'admin','active',?,?,?)`, username, displayName, passwordHash, stamp(now), stamp(now), stamp(now))
	if err != nil {
		return User{}, classifyConstraint(err)
	}
	id, _ := result.LastInsertId()
	if _, err = tx.ExecContext(ctx, `UPDATE system_settings SET value='true',updated_at=? WHERE key='setup_completed'`, stamp(now)); err != nil {
		return User{}, err
	}
	u, err := scanUser(tx.QueryRowContext(ctx, `SELECT `+userColumns+` FROM users WHERE id=?`, id))
	if err != nil {
		return User{}, err
	}
	if err := tx.Commit(); err != nil {
		return User{}, err
	}
	return u, nil
}

func classifyConstraint(err error) error {
	if err != nil && strings.Contains(strings.ToLower(err.Error()), "unique constraint") {
		return ErrDuplicateAccount
	}
	return err
}

func (r *Repository) CreatePendingPlayer(ctx context.Context, steamID, passwordHash, palworldUserID, palworldPlayerID, characterName, accountName string, now time.Time) (User, error) {
	result, err := r.db.ExecContext(ctx, `INSERT INTO users(password_hash,steam_id,palworld_user_id,palworld_player_id,character_name,account_name,role,status,created_at,updated_at,last_seen_at) VALUES(?,?,?,?,?,?,'player','pending',?,?,?)`, passwordHash, steamID, palworldUserID, nullIfEmpty(palworldPlayerID), characterName, accountName, stamp(now), stamp(now), stamp(now))
	if err != nil {
		return User{}, classifyConstraint(err)
	}
	id, _ := result.LastInsertId()
	return r.FindByID(ctx, id)
}
func nullIfEmpty(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func (r *Repository) FindByID(ctx context.Context, id int64) (User, error) {
	u, err := scanUser(r.db.QueryRowContext(ctx, `SELECT `+userColumns+` FROM users WHERE id=?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, ErrNotFound
	}
	return u, err
}
func (r *Repository) FindBySteamID(ctx context.Context, steamID string) (User, error) {
	u, err := scanUser(r.db.QueryRowContext(ctx, `SELECT `+userColumns+` FROM users WHERE steam_id=?`, steamID))
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, ErrNotFound
	}
	return u, err
}
func (r *Repository) FindByUsername(ctx context.Context, username string) (User, error) {
	u, err := scanUser(r.db.QueryRowContext(ctx, `SELECT `+userColumns+` FROM users WHERE username=? COLLATE NOCASE`, username))
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, ErrNotFound
	}
	return u, err
}

func (r *Repository) CreateSession(ctx context.Context, userID int64, hash string, now, expires time.Time) error {
	_, err := r.db.ExecContext(ctx, `INSERT INTO sessions(user_id,token_hash,created_at,expires_at,last_seen_at) VALUES(?,?,?,?,?)`, userID, hash, stamp(now), stamp(expires), stamp(now))
	return err
}
func (r *Repository) UpdateLogin(ctx context.Context, userID int64, now time.Time) error {
	_, err := r.db.ExecContext(ctx, `UPDATE users SET updated_at=?,last_login_at=? WHERE id=?`, stamp(now), stamp(now), userID)
	return err
}
func (r *Repository) Authenticate(ctx context.Context, hash string, now time.Time) (User, error) {
	u, err := scanUser(r.db.QueryRowContext(ctx, `SELECT u.`+strings.ReplaceAll(userColumns, ",", ",u.")+` FROM sessions s JOIN users u ON u.id=s.user_id WHERE s.token_hash=? AND s.revoked_at IS NULL AND s.expires_at>? AND u.status='active'`, hash, stamp(now)))
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, ErrUnauthenticated
	}
	if err == nil {
		_, _ = r.db.ExecContext(ctx, `UPDATE sessions SET last_seen_at=? WHERE token_hash=?`, stamp(now), hash)
	}
	return u, err
}
func (r *Repository) RevokeToken(ctx context.Context, hash string, now time.Time) error {
	_, err := r.db.ExecContext(ctx, `UPDATE sessions SET revoked_at=? WHERE token_hash=? AND revoked_at IS NULL`, stamp(now), hash)
	return err
}
func (r *Repository) RevokeUserSessions(ctx context.Context, userID int64, now time.Time) error {
	_, err := r.db.ExecContext(ctx, `UPDATE sessions SET revoked_at=? WHERE user_id=? AND revoked_at IS NULL`, stamp(now), userID)
	return err
}

func (r *Repository) ListUsers(ctx context.Context, status string) ([]User, error) {
	query := `SELECT ` + userColumns + ` FROM users`
	args := []any{}
	if status != "" {
		query += ` WHERE status=?`
		args = append(args, status)
	}
	query += ` ORDER BY created_at DESC`
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	users := []User{}
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

func activeAdminCount(ctx context.Context, tx *sql.Tx) (int, error) {
	var count int
	err := tx.QueryRowContext(ctx, `SELECT count(*) FROM users WHERE role='admin' AND status='active'`).Scan(&count)
	return count, err
}
func txUser(ctx context.Context, tx *sql.Tx, id int64) (User, error) {
	u, err := scanUser(tx.QueryRowContext(ctx, `SELECT `+userColumns+` FROM users WHERE id=?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, ErrNotFound
	}
	return u, err
}
func (r *Repository) SetStatus(ctx context.Context, currentID, targetID int64, status string, now time.Time) error {
	if status != StatusActive && status != StatusDisabled && status != StatusDeleted {
		return ErrInvalidTransition
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	target, err := txUser(ctx, tx, targetID)
	if err != nil {
		return err
	}
	if (status == StatusDisabled || status == StatusDeleted) && currentID == targetID {
		return ErrUnsafeAdminAction
	}
	if status == StatusActive && target.Status != StatusDisabled {
		return ErrInvalidTransition
	}
	if status == StatusDisabled && target.Status != StatusActive {
		return ErrInvalidTransition
	}
	if status == StatusDeleted && target.Status == StatusDeleted {
		return ErrInvalidTransition
	}
	if (status == StatusDisabled || status == StatusDeleted) && target.Role == RoleAdmin && target.Status == StatusActive {
		count, err := activeAdminCount(ctx, tx)
		if err != nil {
			return err
		}
		if count <= 1 {
			return ErrUnsafeAdminAction
		}
	}
	if status == StatusDeleted {
		_, err = tx.ExecContext(ctx, `UPDATE users SET previous_status=status,status='deleted',deleted_at=?,updated_at=? WHERE id=?`, stamp(now), stamp(now), targetID)
	} else {
		_, err = tx.ExecContext(ctx, `UPDATE users SET status=?,updated_at=? WHERE id=?`, status, stamp(now), targetID)
	}
	if err != nil {
		return err
	}
	if status == StatusDisabled || status == StatusDeleted {
		if _, err = tx.ExecContext(ctx, `UPDATE sessions SET revoked_at=? WHERE user_id=? AND revoked_at IS NULL`, stamp(now), targetID); err != nil {
			return err
		}
	}
	return tx.Commit()
}
func (r *Repository) Restore(ctx context.Context, targetID int64, now time.Time) error {
	result, err := r.db.ExecContext(ctx, `UPDATE users SET status=COALESCE(previous_status,'active'),previous_status=NULL,deleted_at=NULL,updated_at=? WHERE id=? AND status='deleted'`, stamp(now), targetID)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n != 1 {
		return ErrInvalidTransition
	}
	return nil
}
func (r *Repository) Approve(ctx context.Context, targetID int64, actorID *int64, now time.Time) error {
	result, err := r.db.ExecContext(ctx, `UPDATE users SET status='active',approved_at=?,approved_by=?,rejected_at=NULL,rejected_by=NULL,rejection_reason='',updated_at=? WHERE id=? AND status IN ('pending','rejected')`, stamp(now), actorID, stamp(now), targetID)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n != 1 {
		return ErrInvalidTransition
	}
	return nil
}
func (r *Repository) Reject(ctx context.Context, targetID int64, actorID *int64, reason string, now time.Time) error {
	result, err := r.db.ExecContext(ctx, `UPDATE users SET status='rejected',rejected_at=?,rejected_by=?,rejection_reason=?,updated_at=? WHERE id=? AND status='pending'`, stamp(now), actorID, reason, stamp(now), targetID)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n != 1 {
		return ErrInvalidTransition
	}
	return r.RevokeUserSessions(ctx, targetID, now)
}
func (r *Repository) SetRole(ctx context.Context, targetID int64, role string, now time.Time) error {
	if role != RoleAdmin && role != RolePlayer {
		return ErrInvalidTransition
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	target, err := txUser(ctx, tx, targetID)
	if err != nil {
		return err
	}
	if target.Role == RoleAdmin && role == RolePlayer && target.Status == StatusActive {
		count, err := activeAdminCount(ctx, tx)
		if err != nil {
			return err
		}
		if count <= 1 {
			return ErrUnsafeAdminAction
		}
	}
	if _, err = tx.ExecContext(ctx, `UPDATE users SET role=?,updated_at=? WHERE id=?`, role, stamp(now), targetID); err != nil {
		return err
	}
	return tx.Commit()
}
func (r *Repository) SetRoleBySteamID(ctx context.Context, steamID, role string, now time.Time) error {
	u, err := r.FindBySteamID(ctx, steamID)
	if err != nil {
		return err
	}
	return r.SetRole(ctx, u.ID, role, now)
}
func (r *Repository) UpdatePassword(ctx context.Context, userID int64, passwordHash string, keepTokenHash *string, now time.Time) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `UPDATE users SET password_hash=?,updated_at=? WHERE id=?`, passwordHash, stamp(now), userID)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n != 1 {
		return ErrNotFound
	}
	if keepTokenHash == nil {
		_, err = tx.ExecContext(ctx, `UPDATE sessions SET revoked_at=? WHERE user_id=? AND revoked_at IS NULL`, stamp(now), userID)
	} else {
		_, err = tx.ExecContext(ctx, `UPDATE sessions SET revoked_at=? WHERE user_id=? AND token_hash<>? AND revoked_at IS NULL`, stamp(now), userID, *keepTokenHash)
	}
	if err != nil {
		return err
	}
	return tx.Commit()
}
func (r *Repository) Cleanup(ctx context.Context, now time.Time) {
	_, _ = r.db.ExecContext(ctx, `DELETE FROM sessions WHERE expires_at<?`, stamp(now))
	_, _ = r.db.ExecContext(ctx, `DELETE FROM auth_flows WHERE expires_at<?`, stamp(now.Add(-24*time.Hour)))
}

func (r *Repository) ResolveByIdentifier(ctx context.Context, identifier string) (User, error) {
	if isSteamID(identifier) {
		return r.FindBySteamID(ctx, identifier)
	}
	return r.FindByUsername(ctx, identifier)
}

func (r *Repository) UserBySteamIDRequired(ctx context.Context, steamID string) (User, error) {
	return r.FindBySteamID(ctx, steamID)
}
func (r *Repository) UserByUsernameRequired(ctx context.Context, username string) (User, error) {
	return r.FindByUsername(ctx, username)
}

func (r *Repository) DebugPasswordHash(ctx context.Context, id int64) (string, error) {
	var hash sql.NullString
	if err := r.db.QueryRowContext(ctx, `SELECT password_hash FROM users WHERE id=?`, id).Scan(&hash); err != nil {
		return "", err
	}
	if !hash.Valid {
		return "", fmt.Errorf("password is not set")
	}
	return hash.String, nil
}
