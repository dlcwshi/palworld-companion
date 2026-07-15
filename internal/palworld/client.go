package palworld

import "context"

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
	Name      string   `json:"name"`
	PlayerID  string   `json:"playerId"`
	UserID    string   `json:"userId"`
	IP        string   `json:"ip"`
	Ping      *float64 `json:"ping"`
	LocationX *float64 `json:"location_x"`
	LocationY *float64 `json:"location_y"`
	Level     *int     `json:"level"`
}
type Players struct {
	Players []Player `json:"players"`
}

type Client interface {
	GetInfo(context.Context) (Info, error)
	GetMetrics(context.Context) (Metrics, error)
	GetPlayers(context.Context) (Players, error)
}
