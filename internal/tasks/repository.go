package tasks

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

type Repository struct{ db *sql.DB }

func NewRepository(db *sql.DB) *Repository { return &Repository{db: db} }

const taskColumns = `id,title,notes,status,sort_order,source_type,source_id,created_at,updated_at,completed_at,owner_id,created_by,visibility`

func (r *Repository) Create(ctx context.Context, task *Task) error {
	result, err := r.db.ExecContext(ctx, `INSERT INTO tasks(title, notes, status, sort_order, source_type, source_id, created_at, updated_at, completed_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`, task.Title, task.Notes, task.Status, task.SortOrder, task.SourceType, task.SourceID, formatTime(task.CreatedAt), formatTime(task.UpdatedAt), nullableTime(task.CompletedAt))
	if err != nil {
		return fmt.Errorf("create task: %w", err)
	}
	task.ID, err = result.LastInsertId()
	if err != nil {
		return fmt.Errorf("read task id: %w", err)
	}
	return nil
}
func (r *Repository) CreateOwned(ctx context.Context, task *Task) error {
	result, err := r.db.ExecContext(ctx, `INSERT INTO tasks(title,notes,status,sort_order,source_type,source_id,created_at,updated_at,completed_at,owner_id,created_by,visibility) VALUES(?,?,?,?,?,?,?,?,?,?,?,?)`, task.Title, task.Notes, task.Status, task.SortOrder, task.SourceType, task.SourceID, formatTime(task.CreatedAt), formatTime(task.UpdatedAt), nullableTime(task.CompletedAt), task.OwnerID, task.CreatedBy, task.Visibility)
	if err != nil {
		return fmt.Errorf("create task: %w", err)
	}
	task.ID, err = result.LastInsertId()
	return err
}
func (r *Repository) Get(ctx context.Context, id int64) (Task, error) {
	task, err := scanTask(r.db.QueryRowContext(ctx, `SELECT `+taskColumns+` FROM tasks WHERE id = ?`, id))
	if err == sql.ErrNoRows {
		return Task{}, ErrNotFound
	}
	if err != nil {
		return Task{}, fmt.Errorf("get task: %w", err)
	}
	return task, nil
}
func (r *Repository) List(ctx context.Context, options ListOptions) ([]Task, error) {
	query := `SELECT ` + taskColumns + ` FROM tasks`
	args := make([]any, 0, 2)
	if options.Status != "all" {
		query += ` WHERE status = ?`
		args = append(args, options.Status)
	}
	query += ` ORDER BY CASE status WHEN 'pending' THEN 0 ELSE 1 END, sort_order ASC, created_at DESC LIMIT ?`
	args = append(args, options.Limit)
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list tasks: %w", err)
	}
	defer rows.Close()
	result := make([]Task, 0)
	for rows.Next() {
		task, err := scanTask(rows)
		if err != nil {
			return nil, fmt.Errorf("scan task: %w", err)
		}
		result = append(result, task)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list tasks: %w", err)
	}
	return result, nil
}
func scopeWhere(actor Actor, options ListOptions) (string, []any, error) {
	if options.Scope == "" {
		options.Scope = "visible"
	}
	args := []any{}
	var visibility string
	switch options.Scope {
	case "mine":
		visibility = `owner_id = ?`
		args = append(args, actor.ID)
	case "shared":
		visibility = `visibility = 'shared'`
	case "visible":
		visibility = `(visibility = 'shared' OR owner_id = ?)`
		args = append(args, actor.ID)
	case "admin":
		if actor.Role != "admin" {
			return "", nil, ErrNotFound
		}
		visibility = `1=1`
	default:
		return "", nil, ErrInvalidInput
	}
	where := visibility
	if options.Status != "all" {
		where += ` AND status = ?`
		args = append(args, options.Status)
	}
	return where, args, nil
}
func (r *Repository) GetVisible(ctx context.Context, actor Actor, id int64) (Task, error) {
	query := `SELECT ` + taskColumns + ` FROM tasks WHERE id=? AND (visibility='shared' OR owner_id=? OR ?='admin')`
	task, err := scanTask(r.db.QueryRowContext(ctx, query, id, actor.ID, actor.Role))
	if err == sql.ErrNoRows {
		return Task{}, ErrNotFound
	}
	if err != nil {
		return Task{}, err
	}
	r.enrich(ctx, &task, actor)
	return task, nil
}
func (r *Repository) ListVisible(ctx context.Context, actor Actor, options ListOptions) ([]Task, int, error) {
	where, args, err := scopeWhere(actor, options)
	if err != nil {
		return nil, 0, err
	}
	var total int
	if err := r.db.QueryRowContext(ctx, `SELECT count(*) FROM tasks WHERE `+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	query := `SELECT ` + taskColumns + ` FROM tasks WHERE ` + where + ` ORDER BY CASE status WHEN 'pending' THEN 0 ELSE 1 END,sort_order ASC,created_at DESC LIMIT ?`
	args = append(args, options.Limit)
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	out := []Task{}
	for rows.Next() {
		task, e := scanTask(rows)
		if e != nil {
			_ = rows.Close()
			return nil, 0, e
		}
		out = append(out, task)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, 0, err
	}
	if err := rows.Close(); err != nil {
		return nil, 0, err
	}
	for i := range out {
		r.enrich(ctx, &out[i], actor)
	}
	return out, total, nil
}
func (r *Repository) enrich(ctx context.Context, task *Task, actor Actor) {
	task.CanManage = actor.Role == "admin" || (task.Visibility == VisibilityPersonal && task.OwnerID != nil && *task.OwnerID == actor.ID) || (task.Visibility == VisibilityShared && task.CreatedBy != nil && *task.CreatedBy == actor.ID)
	if task.OwnerID == nil {
		return
	}
	var name, status string
	if err := r.db.QueryRowContext(ctx, `SELECT character_name,status FROM users WHERE id=?`, *task.OwnerID).Scan(&name, &status); err == nil {
		if status == "deleted" {
			name = "已删除玩家"
		}
		task.Owner = &OwnerSummary{ID: *task.OwnerID, CharacterName: name, Status: status}
	}
}
func (r *Repository) Count(ctx context.Context, status string) (int, error) {
	query := `SELECT count(*) FROM tasks`
	args := []any{}
	if status != "all" {
		query += ` WHERE status = ?`
		args = append(args, status)
	}
	var count int
	if err := r.db.QueryRowContext(ctx, query, args...).Scan(&count); err != nil {
		return 0, fmt.Errorf("count tasks: %w", err)
	}
	return count, nil
}
func (r *Repository) Update(ctx context.Context, task Task) error {
	result, err := r.db.ExecContext(ctx, `UPDATE tasks SET title=?, notes=?, status=?, sort_order=?, updated_at=?, completed_at=? WHERE id=?`, task.Title, task.Notes, task.Status, task.SortOrder, formatTime(task.UpdatedAt), nullableTime(task.CompletedAt), task.ID)
	if err != nil {
		return fmt.Errorf("update task: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read update result: %w", err)
	}
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}
func (r *Repository) Delete(ctx context.Context, id int64) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM tasks WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete task: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read delete result: %w", err)
	}
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

type scanner interface{ Scan(...any) error }

func scanTask(row scanner) (Task, error) {
	var task Task
	var created, updated string
	var completed sql.NullString
	if err := row.Scan(&task.ID, &task.Title, &task.Notes, &task.Status, &task.SortOrder, &task.SourceType, &task.SourceID, &created, &updated, &completed, &task.OwnerID, &task.CreatedBy, &task.Visibility); err != nil {
		return Task{}, err
	}
	var err error
	if task.CreatedAt, err = time.Parse(time.RFC3339Nano, created); err != nil {
		return Task{}, err
	}
	if task.UpdatedAt, err = time.Parse(time.RFC3339Nano, updated); err != nil {
		return Task{}, err
	}
	if completed.Valid {
		value, err := time.Parse(time.RFC3339Nano, completed.String)
		if err != nil {
			return Task{}, err
		}
		task.CompletedAt = &value
	}
	return task, nil
}
func formatTime(value time.Time) string { return value.UTC().Format(time.RFC3339Nano) }
func nullableTime(value *time.Time) any {
	if value == nil {
		return nil
	}
	return formatTime(*value)
}
