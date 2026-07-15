package tasks

import (
	"errors"
	"time"
)

const (
	StatusPending      = "pending"
	StatusCompleted    = "completed"
	SourceManual       = "manual"
	VisibilityPersonal = "personal"
	VisibilityShared   = "shared"
	MaxTitleLength     = 200
	MaxNotesLength     = 4000
)

var (
	ErrNotFound     = errors.New("task not found")
	ErrInvalidInput = errors.New("invalid task input")
)

type Task struct {
	ID          int64         `json:"id"`
	Title       string        `json:"title"`
	Notes       string        `json:"notes"`
	Status      string        `json:"status"`
	SortOrder   int           `json:"sortOrder"`
	SourceType  string        `json:"sourceType"`
	SourceID    *int64        `json:"sourceId"`
	CreatedAt   time.Time     `json:"createdAt"`
	UpdatedAt   time.Time     `json:"updatedAt"`
	CompletedAt *time.Time    `json:"completedAt"`
	OwnerID     *int64        `json:"-"`
	CreatedBy   *int64        `json:"-"`
	Visibility  string        `json:"visibility"`
	Owner       *OwnerSummary `json:"owner"`
	CanManage   bool          `json:"canManage"`
}
type OwnerSummary struct {
	ID            int64  `json:"id"`
	CharacterName string `json:"characterName"`
	Status        string `json:"status"`
}
type Actor struct {
	ID   int64
	Role string
}
type ListOptions struct {
	Status string
	Limit  int
	Scope  string
}
type CreateInput struct {
	Title      string
	Notes      string
	Visibility string
}
type UpdateInput struct {
	Title     *string
	Notes     *string
	Status    *string
	SortOrder *int
}
