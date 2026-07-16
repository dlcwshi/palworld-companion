package roster

import "time"

const (
	StatusOnline  = "online"
	StatusOffline = "offline"
	StatusUnknown = "unknown"
)

type Player struct {
	ID               int64
	PalworldUserID   string
	PalworldPlayerID *string
	CharacterName    string
	Level            int
	IsOnline         bool
	FirstSeenAt      time.Time
	LastOnlineAt     time.Time
	UpdatedAt        time.Time
}

type Counts struct {
	Total            int  `json:"total"`
	CurrentOnline    *int `json:"currentOnline"`
	CurrentOffline   *int `json:"currentOffline"`
	LastKnownOnline  int  `json:"lastKnownOnline"`
	LastKnownOffline int  `json:"lastKnownOffline"`
}

type Response struct {
	Available          bool           `json:"available"`
	Cached             bool           `json:"cached"`
	Stale              bool           `json:"stale"`
	CurrentStatusKnown bool           `json:"currentStatusKnown"`
	UpdatedAt          *time.Time     `json:"updatedAt"`
	Error              *string        `json:"error"`
	Counts             Counts         `json:"counts"`
	Players            []PublicPlayer `json:"players"`
}

type PublicPlayer struct {
	Name            string          `json:"name"`
	Level           int             `json:"level"`
	Status          string          `json:"status"`
	LastKnownStatus string          `json:"lastKnownStatus"`
	LastOnlineAt    time.Time       `json:"lastOnlineAt"`
	Ping            *float64        `json:"ping"`
	Position        *PublicPosition `json:"position"`
}

type PublicPosition struct {
	X float64  `json:"x"`
	Y float64  `json:"y"`
	Z *float64 `json:"z,omitempty"`
}
