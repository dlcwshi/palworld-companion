package auth

import (
	"errors"
	"time"
)

const (
	RoleAdmin         = "admin"
	RolePlayer        = "player"
	StatusActive      = "active"
	StatusDisabled    = "disabled"
	StatusDeleted     = "deleted"
	SessionCookieName = "palworld_companion_session"
	StateCookieName   = "palworld_companion_openid_state"
)

var (
	ErrUnauthenticated   = errors.New("authentication required")
	ErrForbidden         = errors.New("forbidden")
	ErrAuthDisabled      = errors.New("Steam authentication is not configured")
	ErrInvalidFlow       = errors.New("invalid or expired Steam login flow")
	ErrPlayerOffline     = errors.New("请先进入本 Palworld 服务器并保持在线，然后重新使用 Steam 登录。")
	ErrUpstream          = errors.New("Palworld API unavailable; account was not created")
	ErrAccountDisabled   = errors.New("account is disabled")
	ErrAccountDeleted    = errors.New("account is deleted")
	ErrUnsafeAdminAction = errors.New("operation would disable or delete the current or last active administrator")
)

type User struct {
	ID               int64      `json:"id"`
	SteamID          string     `json:"steamId"`
	PalworldUserID   string     `json:"-"`
	PalworldPlayerID string     `json:"-"`
	CharacterName    string     `json:"characterName"`
	AccountName      string     `json:"accountName"`
	Role             string     `json:"role"`
	Status           string     `json:"status"`
	CreatedAt        time.Time  `json:"createdAt"`
	UpdatedAt        time.Time  `json:"-"`
	LastLoginAt      time.Time  `json:"lastLoginAt"`
	LastSeenAt       *time.Time `json:"lastSeenAt"`
	DeletedAt        *time.Time `json:"deletedAt,omitempty"`
}

type Flow struct {
	ID         int64
	StateHash  string
	ReturnPath string
	CreatedAt  time.Time
	ExpiresAt  time.Time
	ConsumedAt *time.Time
}

type Session struct {
	ID         int64
	UserID     int64
	TokenHash  string
	CreatedAt  time.Time
	ExpiresAt  time.Time
	LastSeenAt time.Time
	RevokedAt  *time.Time
}
