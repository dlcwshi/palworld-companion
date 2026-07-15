package tasks

import (
	"context"
	"fmt"
	"strings"
	"time"
)

type Service struct {
	repo *Repository
	now  func() time.Time
}

type ListResult struct {
	Tasks []Task `json:"tasks"`
	Total int    `json:"total"`
}

func NewService(repo *Repository) *Service { return &Service{repo: repo, now: time.Now} }

func (s *Service) Create(ctx context.Context, input CreateInput) (Task, error) {
	title, notes, err := validateText(input.Title, input.Notes)
	if err != nil {
		return Task{}, err
	}
	now := s.now().UTC()
	task := Task{Title: title, Notes: notes, Status: StatusPending, SourceType: SourceManual, CreatedAt: now, UpdatedAt: now}
	if err := s.repo.Create(ctx, &task); err != nil {
		return Task{}, err
	}
	return task, nil
}

func (s *Service) Get(ctx context.Context, id int64) (Task, error) {
	if id <= 0 {
		return Task{}, ErrNotFound
	}
	return s.repo.Get(ctx, id)
}

func (s *Service) List(ctx context.Context, options ListOptions) (ListResult, error) {
	if options.Status == "" {
		options.Status = "all"
	}
	if !validStatusFilter(options.Status) {
		return ListResult{}, fmt.Errorf("%w: status must be pending, completed, or all", ErrInvalidInput)
	}
	if options.Limit == 0 {
		options.Limit = 100
	}
	if options.Limit < 1 || options.Limit > 100 {
		return ListResult{}, fmt.Errorf("%w: limit must be between 1 and 100", ErrInvalidInput)
	}
	items, err := s.repo.List(ctx, options)
	if err != nil {
		return ListResult{}, err
	}
	total, err := s.repo.Count(ctx, options.Status)
	if err != nil {
		return ListResult{}, err
	}
	return ListResult{Tasks: items, Total: total}, nil
}

func (s *Service) Update(ctx context.Context, id int64, input UpdateInput) (Task, error) {
	if input.Title == nil && input.Notes == nil && input.Status == nil && input.SortOrder == nil {
		return Task{}, fmt.Errorf("%w: no fields provided", ErrInvalidInput)
	}
	task, err := s.Get(ctx, id)
	if err != nil {
		return Task{}, err
	}
	if input.Title != nil {
		task.Title = strings.TrimSpace(*input.Title)
	}
	if input.Notes != nil {
		task.Notes = strings.TrimSpace(*input.Notes)
	}
	if _, _, err := validateText(task.Title, task.Notes); err != nil {
		return Task{}, err
	}
	if input.Status != nil {
		if !validTaskStatus(*input.Status) {
			return Task{}, fmt.Errorf("%w: status must be pending or completed", ErrInvalidInput)
		}
		if task.Status != *input.Status {
			task.Status = *input.Status
			if task.Status == StatusCompleted {
				completed := s.now().UTC()
				task.CompletedAt = &completed
			} else {
				task.CompletedAt = nil
			}
		}
	}
	if input.SortOrder != nil {
		task.SortOrder = *input.SortOrder
	}
	task.UpdatedAt = s.now().UTC()
	if err := s.repo.Update(ctx, task); err != nil {
		return Task{}, err
	}
	return task, nil
}

func (s *Service) Delete(ctx context.Context, id int64) error {
	if id <= 0 {
		return ErrNotFound
	}
	return s.repo.Delete(ctx, id)
}

func validateText(title, notes string) (string, string, error) {
	title, notes = strings.TrimSpace(title), strings.TrimSpace(notes)
	if title == "" {
		return "", "", fmt.Errorf("%w: title is required", ErrInvalidInput)
	}
	if len([]rune(title)) > MaxTitleLength {
		return "", "", fmt.Errorf("%w: title exceeds %d characters", ErrInvalidInput, MaxTitleLength)
	}
	if len([]rune(notes)) > MaxNotesLength {
		return "", "", fmt.Errorf("%w: notes exceeds %d characters", ErrInvalidInput, MaxNotesLength)
	}
	return title, notes, nil
}

func validTaskStatus(value string) bool   { return value == StatusPending || value == StatusCompleted }
func validStatusFilter(value string) bool { return value == "all" || validTaskStatus(value) }
