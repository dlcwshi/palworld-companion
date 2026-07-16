package palworld

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

type Info struct {
	Version     *string `json:"version"`
	ServerName  string  `json:"servername"`
	Description *string `json:"description"`
}
type Metrics struct {
	ServerFPS      float64 `json:"serverfps"`
	CurrentPlayers int     `json:"currentplayernum"`
	MaxPlayers     int     `json:"maxplayernum"`
	Uptime         int64   `json:"uptime"`
	BaseCampCount  int     `json:"basecampnum"`
	Days           int     `json:"days"`
}
type Player struct {
	Name        string   `json:"name"`
	PlayerID    string   `json:"playerId"`
	UserID      string   `json:"userId"`
	IP          string   `json:"ip"`
	AccountName string   `json:"accountName"`
	Ping        *float64 `json:"ping"`
	LocationX   *float64 `json:"location_x"`
	LocationY   *float64 `json:"location_y"`
	LocationZ   *float64 `json:"location_z"`
	Level       *int     `json:"level"`
}
type Players struct {
	Players []Player `json:"players"`
}

var (
	ErrInvalidPlayersSnapshot    = errors.New("invalid Palworld players snapshot")
	ErrPlayerIdentityUnavailable = errors.New("Palworld player identity unavailable")
)

func (p *Players) UnmarshalJSON(data []byte) error {
	var object map[string]json.RawMessage
	if err := json.Unmarshal(data, &object); err != nil {
		return fmt.Errorf("%w: decode object: %v", ErrInvalidPlayersSnapshot, err)
	}
	raw, ok := object["players"]
	if !ok {
		return fmt.Errorf("%w: players field is missing", ErrInvalidPlayersSnapshot)
	}
	if string(raw) == "null" {
		return fmt.Errorf("%w: players field is null", ErrInvalidPlayersSnapshot)
	}
	var players []Player
	if err := json.Unmarshal(raw, &players); err != nil {
		return fmt.Errorf("%w: players field is not an array", ErrInvalidPlayersSnapshot)
	}
	p.Players = players
	return nil
}

func SteamIDFromUserID(userID string) (string, error) {
	if !strings.HasPrefix(userID, "steam_") {
		return "", ErrPlayerIdentityUnavailable
	}
	steamID := strings.TrimPrefix(userID, "steam_")
	if steamID == "" {
		return "", ErrPlayerIdentityUnavailable
	}
	for _, r := range steamID {
		if r < '0' || r > '9' {
			return "", ErrPlayerIdentityUnavailable
		}
	}
	value, err := strconv.ParseUint(steamID, 10, 64)
	if err != nil || value == 0 {
		return "", ErrPlayerIdentityUnavailable
	}
	return steamID, nil
}

func ValidatePlayers(snapshot Players) error {
	userIDs := make(map[string]struct{}, len(snapshot.Players))
	playerIDs := make(map[string]string, len(snapshot.Players))
	for _, player := range snapshot.Players {
		if _, err := SteamIDFromUserID(player.UserID); err != nil {
			return fmt.Errorf("%w: %w", ErrInvalidPlayersSnapshot, err)
		}
		if strings.TrimSpace(player.Name) == "" {
			return fmt.Errorf("%w: player character name is empty", ErrInvalidPlayersSnapshot)
		}
		if _, exists := userIDs[player.UserID]; exists {
			return fmt.Errorf("%w: duplicate userId", ErrInvalidPlayersSnapshot)
		}
		userIDs[player.UserID] = struct{}{}
		if player.PlayerID != "" {
			if previous, exists := playerIDs[player.PlayerID]; exists && previous != player.UserID {
				return fmt.Errorf("%w: duplicate playerId", ErrInvalidPlayersSnapshot)
			}
			playerIDs[player.PlayerID] = player.UserID
		}
	}
	return nil
}

type Client interface {
	GetInfo(context.Context) (Info, error)
	GetMetrics(context.Context) (Metrics, error)
	GetPlayers(context.Context) (Players, error)
}

// GetPlayersFreshForIdentityBinding bypasses the server-status cache. Callers
// must treat the returned identity fields as internal and never serialize them.
func GetPlayersFreshForIdentityBinding(ctx context.Context, client Client) (Players, error) {
	players, err := client.GetPlayers(ctx)
	if err != nil {
		return Players{}, err
	}
	if err := ValidatePlayers(players); err != nil {
		return Players{}, err
	}
	return players, nil
}
