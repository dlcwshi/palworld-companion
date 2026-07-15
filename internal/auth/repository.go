package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

type Repository struct{ db *sql.DB }

func NewRepository(db *sql.DB) *Repository { return &Repository{db: db} }
func stamp(t time.Time) string             { return t.UTC().Format(time.RFC3339Nano) }

func (r *Repository) CreateFlow(ctx context.Context, hash, returnPath string, now, expires time.Time) error {
	_, err := r.db.ExecContext(ctx, `INSERT INTO auth_flows(state_hash,return_path,created_at,expires_at) VALUES(?,?,?,?)`, hash, returnPath, stamp(now), stamp(expires))
	return err
}

func (r *Repository) GetFlow(ctx context.Context, hash string) (Flow, error) {
	var f Flow
	var created, expires string
	var consumed sql.NullString
	err := r.db.QueryRowContext(ctx, `SELECT id,state_hash,return_path,created_at,expires_at,consumed_at FROM auth_flows WHERE state_hash=?`, hash).Scan(&f.ID, &f.StateHash, &f.ReturnPath, &created, &expires, &consumed)
	if errors.Is(err, sql.ErrNoRows) {
		return Flow{}, ErrInvalidFlow
	}
	if err != nil {
		return Flow{}, err
	}
	f.CreatedAt, _ = time.Parse(time.RFC3339Nano, created)
	f.ExpiresAt, _ = time.Parse(time.RFC3339Nano, expires)
	if consumed.Valid {
		v, _ := time.Parse(time.RFC3339Nano, consumed.String)
		f.ConsumedAt = &v
	}
	return f, nil
}

func (r *Repository) ConsumeFlow(ctx context.Context, id int64, now time.Time) error {
	result, err := r.db.ExecContext(ctx, `UPDATE auth_flows SET consumed_at=? WHERE id=? AND consumed_at IS NULL AND expires_at>?`, stamp(now), id, stamp(now))
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n != 1 {
		return ErrInvalidFlow
	}
	return nil
}

func scanUser(row interface{ Scan(...any) error }) (User, error) {
	var u User
	var created, updated, lastLogin string
	var lastSeen, deleted sql.NullString
	err := row.Scan(&u.ID, &u.SteamID, &u.PalworldUserID, &u.PalworldPlayerID, &u.CharacterName, &u.AccountName, &u.Role, &u.Status, &created, &updated, &lastLogin, &lastSeen, &deleted)
	if err != nil {
		return User{}, err
	}
	u.CreatedAt, _ = time.Parse(time.RFC3339Nano, created)
	u.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updated)
	u.LastLoginAt, _ = time.Parse(time.RFC3339Nano, lastLogin)
	if lastSeen.Valid {
		v, _ := time.Parse(time.RFC3339Nano, lastSeen.String)
		u.LastSeenAt = &v
	}
	if deleted.Valid {
		v, _ := time.Parse(time.RFC3339Nano, deleted.String)
		u.DeletedAt = &v
	}
	return u, nil
}

const userColumns = `id,steam_id,palworld_user_id,palworld_player_id,character_name,account_name,role,status,created_at,updated_at,last_login_at,last_seen_at,deleted_at`

func (r *Repository) FindBySteamID(ctx context.Context, steamID string) (User, error) {
	u, err := scanUser(r.db.QueryRowContext(ctx, `SELECT `+userColumns+` FROM users WHERE steam_id=?`, steamID))
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, sql.ErrNoRows
	}
	return u, err
}

func (r *Repository) FindByID(ctx context.Context, id int64) (User, error) {
	u, err := scanUser(r.db.QueryRowContext(ctx, `SELECT `+userColumns+` FROM users WHERE id=?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, ErrUnauthenticated
	}
	return u, err
}

func (r *Repository) CreateUser(ctx context.Context, u User) (User, error) {
	_, err := r.db.ExecContext(ctx, `INSERT INTO users(steam_id,palworld_user_id,palworld_player_id,character_name,account_name,role,status,created_at,updated_at,last_login_at,last_seen_at) VALUES(?,?,?,?,?,?,?,?,?,?,?) ON CONFLICT(steam_id) DO NOTHING`, u.SteamID, u.PalworldUserID, u.PalworldPlayerID, u.CharacterName, u.AccountName, u.Role, u.Status, stamp(u.CreatedAt), stamp(u.UpdatedAt), stamp(u.LastLoginAt), stamp(*u.LastSeenAt))
	if err != nil {
		return User{}, err
	}
	return r.FindBySteamID(ctx, u.SteamID)
}

func (r *Repository) UpdateLogin(ctx context.Context, u User, now time.Time, updateIdentity bool) error {
	if updateIdentity {
		_, err := r.db.ExecContext(ctx, `UPDATE users SET palworld_player_id=?,character_name=?,account_name=?,role=CASE WHEN ?='admin' THEN 'admin' ELSE role END,updated_at=?,last_login_at=?,last_seen_at=? WHERE id=?`, u.PalworldPlayerID, u.CharacterName, u.AccountName, u.Role, stamp(now), stamp(now), stamp(now), u.ID)
		return err
	}
	_, err := r.db.ExecContext(ctx, `UPDATE users SET role=CASE WHEN ?='admin' THEN 'admin' ELSE role END,updated_at=?,last_login_at=? WHERE id=?`, u.Role, stamp(now), stamp(now), u.ID)
	return err
}

func (r *Repository) CreateSession(ctx context.Context, userID int64, hash string, now, expires time.Time) error {
	_, err := r.db.ExecContext(ctx, `INSERT INTO sessions(user_id,token_hash,created_at,expires_at,last_seen_at) VALUES(?,?,?,?,?)`, userID, hash, stamp(now), stamp(expires), stamp(now))
	return err
}

func (r *Repository) Authenticate(ctx context.Context, hash string, now time.Time) (User, error) {
	u, err := scanUser(r.db.QueryRowContext(ctx, `SELECT u.`+"id,u.steam_id,u.palworld_user_id,u.palworld_player_id,u.character_name,u.account_name,u.role,u.status,u.created_at,u.updated_at,u.last_login_at,u.last_seen_at,u.deleted_at"+` FROM sessions s JOIN users u ON u.id=s.user_id WHERE s.token_hash=? AND s.revoked_at IS NULL AND s.expires_at>? AND u.status='active'`, hash, stamp(now)))
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
func (r *Repository) Cleanup(ctx context.Context, now time.Time) {
	_, _ = r.db.ExecContext(ctx, `DELETE FROM sessions WHERE expires_at<?`, stamp(now))
	_, _ = r.db.ExecContext(ctx, `DELETE FROM auth_flows WHERE expires_at<?`, stamp(now.Add(-24*time.Hour)))
}

func (r *Repository) ListUsers(ctx context.Context) ([]User, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT `+userColumns+` FROM users ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []User{}
	for rows.Next() {
		u, e := scanUser(rows)
		if e != nil {
			return nil, e
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

func (r *Repository) SetRoleBySteamID(ctx context.Context, steamID, role string, now time.Time) error {
	if role != RoleAdmin && role != RolePlayer {
		return fmt.Errorf("invalid role %q", role)
	}
	result, err := r.db.ExecContext(ctx, `UPDATE users SET role=?,updated_at=? WHERE steam_id=?`, role, stamp(now), steamID)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("user with SteamID %s not found", steamID)
	}
	return nil
}

func (r *Repository) SetStatus(ctx context.Context, currentID, targetID int64, status string, now time.Time) error {
	target, err := r.FindByID(ctx, targetID)
	if err != nil {
		return err
	}
	if (status == StatusDisabled || status == StatusDeleted) && targetID == currentID {
		return ErrUnsafeAdminAction
	}
	if (status == StatusDisabled || status == StatusDeleted) && target.Role == RoleAdmin && target.Status == StatusActive {
		var n int
		if err := r.db.QueryRowContext(ctx, `SELECT count(*) FROM users WHERE role='admin' AND status='active'`).Scan(&n); err != nil {
			return err
		}
		if n <= 1 {
			return ErrUnsafeAdminAction
		}
	}
	var deleted any = nil
	if status == StatusDeleted {
		deleted = stamp(now)
	}
	_, err = r.db.ExecContext(ctx, `UPDATE users SET status=?,deleted_at=?,updated_at=? WHERE id=?`, status, deleted, stamp(now), targetID)
	if err == nil && (status == StatusDisabled || status == StatusDeleted) {
		err = r.RevokeUserSessions(ctx, targetID, now)
	}
	return err
}
