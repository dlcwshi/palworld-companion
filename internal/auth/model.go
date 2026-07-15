package auth

import (
	"errors"
	"time"
)

const (
	RoleAdmin         = "admin"
	RolePlayer        = "player"
	StatusPending     = "pending"
	StatusActive      = "active"
	StatusDisabled    = "disabled"
	StatusRejected    = "rejected"
	StatusDeleted     = "deleted"
	SessionCookieName = "palworld_companion_session"
	StateCookieName   = "palworld_companion_openid_state"
)

var (
	ErrUnauthenticated     = errors.New("authentication required")
	ErrForbidden           = errors.New("forbidden")
	ErrAlreadyInitialized  = errors.New("initial setup has already been completed")
	ErrSetupRequired       = errors.New("initial setup is required")
	ErrInvalidCredentials  = errors.New("invalid account or password")
	ErrInvalidInput        = errors.New("invalid input")
	ErrApprovalPending     = errors.New("account is waiting for administrator approval")
	ErrAccountDisabled     = errors.New("account is disabled")
	ErrApplicationRejected = errors.New("account application was rejected")
	ErrAccountDeleted      = errors.New("account is deleted")
	ErrPlayerOffline       = errors.New("player is not currently online")
	ErrUpstream            = errors.New("Palworld API unavailable")
	ErrDuplicateAccount    = errors.New("an account already exists for this player")
	ErrNotFound            = errors.New("user not found")
	ErrInvalidTransition   = errors.New("invalid account status transition")
	ErrUnsafeAdminAction   = errors.New("operation would modify the current or last active administrator")
	ErrInvalidFlow         = errors.New("Steam OpenID login is disabled")
	ErrAuthDisabled        = errors.New("Steam OpenID login is disabled")
)

type User struct {
	ID               int64      `json:"id"`
	Username         *string    `json:"username"`
	DisplayName      string     `json:"displayName"`
	PasswordHash     *string    `json:"-"`
	SteamID          *string    `json:"steamId"`
	PalworldUserID   *string    `json:"palworldUserId"`
	PalworldPlayerID *string    `json:"palworldPlayerId,omitempty"`
	CharacterName    string     `json:"characterName"`
	AccountName      string     `json:"accountName"`
	Role             string     `json:"role"`
	Status           string     `json:"status"`
	PreviousStatus   *string    `json:"-"`
	CreatedAt        time.Time  `json:"createdAt"`
	UpdatedAt        time.Time  `json:"updatedAt"`
	LastLoginAt      *time.Time `json:"lastLoginAt"`
	LastSeenAt       *time.Time `json:"lastSeenAt"`
	DeletedAt        *time.Time `json:"deletedAt,omitempty"`
	ApprovedAt       *time.Time `json:"approvedAt"`
	ApprovedBy       *int64     `json:"approvedBy"`
	RejectedAt       *time.Time `json:"rejectedAt"`
	RejectedBy       *int64     `json:"rejectedBy"`
	RejectionReason  string     `json:"rejectionReason,omitempty"`
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

type Flow struct {
	ID         int64
	StateHash  string
	ReturnPath string
	CreatedAt  time.Time
	ExpiresAt  time.Time
	ConsumedAt *time.Time
}
