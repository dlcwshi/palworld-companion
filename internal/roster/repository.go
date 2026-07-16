package roster

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/dlcwshi/palworld-companion/internal/palworld"
)

const lastSuccessSetting = "player_roster_last_success_at"

type Repository struct{ db *sql.DB }

func NewRepository(db *sql.DB) *Repository { return &Repository{db: db} }

func timestamp(value time.Time) string { return value.UTC().Format(time.RFC3339Nano) }

func parseTimestamp(value string) (time.Time, error) {
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse roster timestamp: %w", err)
	}
	return parsed, nil
}

func (r *Repository) ApplySnapshot(ctx context.Context, snapshot palworld.Players, completedAt time.Time) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin roster synchronization: %w", err)
	}
	defer tx.Rollback()

	stamp := timestamp(completedAt)
	if _, err = tx.ExecContext(ctx, `UPDATE player_roster SET is_online=0,updated_at=?`, stamp); err != nil {
		return fmt.Errorf("mark previous roster offline: %w", err)
	}
	for _, player := range snapshot.Players {
		level := 0
		if player.Level != nil {
			level = *player.Level
		}
		var playerID any
		if player.PlayerID != "" {
			playerID = player.PlayerID
		}
		if _, err = tx.ExecContext(ctx, `
INSERT INTO player_roster(
	palworld_user_id,palworld_player_id,character_name,level,is_online,
	first_seen_at,last_online_at,updated_at
) VALUES(?,?,?,?,1,?,?,?)
ON CONFLICT(palworld_user_id) DO UPDATE SET
	palworld_player_id=CASE
		WHEN excluded.palworld_player_id IS NOT NULL THEN excluded.palworld_player_id
		ELSE player_roster.palworld_player_id
	END,
	character_name=excluded.character_name,
	level=excluded.level,
	is_online=1,
	last_online_at=excluded.last_online_at,
	updated_at=excluded.updated_at
`, player.UserID, playerID, player.Name, level, stamp, stamp, stamp); err != nil {
			return fmt.Errorf("upsert roster player: %w", err)
		}
		if _, err = tx.ExecContext(ctx, `
UPDATE users SET
	character_name=?,
	palworld_player_id=CASE WHEN ? IS NOT NULL THEN ? ELSE palworld_player_id END,
	account_name=?,
	last_seen_at=?,
	updated_at=?
WHERE palworld_user_id=?
`, player.Name, playerID, playerID, player.AccountName, stamp, stamp, player.UserID); err != nil {
			return fmt.Errorf("update bound user from roster: %w", err)
		}
	}
	if _, err = tx.ExecContext(ctx, `
INSERT INTO system_settings(key,value,updated_at) VALUES(?,?,?)
ON CONFLICT(key) DO UPDATE SET value=excluded.value,updated_at=excluded.updated_at
`, lastSuccessSetting, stamp, stamp); err != nil {
		return fmt.Errorf("update roster synchronization time: %w", err)
	}
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit roster synchronization: %w", err)
	}
	return nil
}

func (r *Repository) State(ctx context.Context) ([]Player, *time.Time, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT id,palworld_user_id,palworld_player_id,character_name,level,is_online,
       first_seen_at,last_online_at,updated_at
FROM player_roster
`)
	if err != nil {
		return nil, nil, fmt.Errorf("query player roster: %w", err)
	}
	defer rows.Close()
	players := make([]Player, 0)
	for rows.Next() {
		var player Player
		var playerID sql.NullString
		var online int
		var firstSeen, lastOnline, updated string
		if err := rows.Scan(&player.ID, &player.PalworldUserID, &playerID, &player.CharacterName, &player.Level, &online, &firstSeen, &lastOnline, &updated); err != nil {
			return nil, nil, fmt.Errorf("scan player roster: %w", err)
		}
		if playerID.Valid {
			value := playerID.String
			player.PalworldPlayerID = &value
		}
		player.IsOnline = online == 1
		if player.FirstSeenAt, err = parseTimestamp(firstSeen); err != nil {
			return nil, nil, err
		}
		if player.LastOnlineAt, err = parseTimestamp(lastOnline); err != nil {
			return nil, nil, err
		}
		if player.UpdatedAt, err = parseTimestamp(updated); err != nil {
			return nil, nil, err
		}
		players = append(players, player)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("iterate player roster: %w", err)
	}

	var raw string
	err = r.db.QueryRowContext(ctx, `SELECT value FROM system_settings WHERE key=?`, lastSuccessSetting).Scan(&raw)
	if err != nil && err != sql.ErrNoRows {
		return nil, nil, fmt.Errorf("read roster synchronization time: %w", err)
	}
	if err == sql.ErrNoRows {
		return players, nil, nil
	}
	parsed, err := parseTimestamp(raw)
	if err != nil {
		return nil, nil, err
	}
	return players, &parsed, nil
}
