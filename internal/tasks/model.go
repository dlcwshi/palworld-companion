package tasks

import (
	"errors"
	"time"
)

const (
	StatusPending   = "pending"
	StatusCompleted = "completed"
	SourceManual    = "manual"
	MaxTitleLength  = 200
	MaxNotesLength  = 4000
)

var (
	ErrNotFound     = errors.New("task not found")
	ErrInvalidInput = errors.New("invalid task input")
)

type Task struct {
	ID          int64      `json:"id"`
	Title       string     `json:"title"`
	Notes       string     `json:"notes"`
	Status      string     `json:"status"`
	SortOrder   int        `json:"sortOrder"`
	SourceType  string     `json:"sourceType"`
	SourceID    *int64     `json:"sourceId"`
	CreatedAt   time.Time  `json:"createdAt"`
	UpdatedAt   time.Time  `json:"updatedAt"`
	CompletedAt *time.Time `json:"completedAt"`
}
type ListOptions struct {
	Status string
	Limit  int
}
type CreateInput struct {
	Title string
	Notes string
}
type UpdateInput struct {
	Title     *string
	Notes     *string
	Status    *string
	SortOrder *int
}
