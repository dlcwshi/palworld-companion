package tasks

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/dlcwshi/palworld-companion/internal/storage"
)

func testService(t *testing.T) (*Service, func()) {
	t.Helper()
	db, err := storage.Open(filepath.Join(t.TempDir(), "tasks.db"))
	if err != nil {
		t.Fatal(err)
	}
	return NewService(NewRepository(db.SQL())), func() { _ = db.Close() }
}

func TestTaskWorkflow(t *testing.T) {
	service, closeDB := testService(t)
	defer closeDB()
	ctx := context.Background()
	created, err := service.Create(ctx, CreateInput{Title: "  Prepare materials  ", Notes: "  ore  "})
	if err != nil {
		t.Fatal(err)
	}
	if created.Title != "Prepare materials" || created.Status != StatusPending || created.ID == 0 {
		t.Fatalf("created=%+v", created)
	}
	got, err := service.Get(ctx, created.ID)
	if err != nil || got.Title != created.Title {
		t.Fatalf("got=%+v err=%v", got, err)
	}
	completed := StatusCompleted
	title := "Updated"
	order := -1
	updated, err := service.Update(ctx, created.ID, UpdateInput{Title: &title, Status: &completed, SortOrder: &order})
	if err != nil || updated.CompletedAt == nil || updated.Status != StatusCompleted {
		t.Fatalf("updated=%+v err=%v", updated, err)
	}
	pending := StatusPending
	restored, err := service.Update(ctx, created.ID, UpdateInput{Status: &pending})
	if err != nil || restored.CompletedAt != nil {
		t.Fatalf("restored=%+v err=%v", restored, err)
	}
	if err := service.Delete(ctx, created.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Get(ctx, created.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("get deleted err=%v", err)
	}
}

func TestTaskValidationAndNotFound(t *testing.T) {
	service, closeDB := testService(t)
	defer closeDB()
	ctx := context.Background()
	if _, err := service.Create(ctx, CreateInput{Title: "   "}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("empty title err=%v", err)
	}
	if _, err := service.Create(ctx, CreateInput{Title: string(make([]rune, MaxTitleLength+1))}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("long title err=%v", err)
	}
	created, err := service.Create(ctx, CreateInput{Title: "valid"})
	if err != nil {
		t.Fatal(err)
	}
	invalid := "invalid"
	if _, err := service.Update(ctx, created.ID, UpdateInput{Status: &invalid}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("invalid status err=%v", err)
	}
	if err := service.Delete(ctx, 9999); !errors.Is(err, ErrNotFound) {
		t.Fatalf("delete missing err=%v", err)
	}
}

func TestTaskPersistsAfterDatabaseReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "persistent.db")
	db, err := storage.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	service := NewService(NewRepository(db.SQL()))
	created, err := service.Create(context.Background(), CreateInput{Title: "survives restart"})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err = storage.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	got, err := NewService(NewRepository(db.SQL())).Get(context.Background(), created.ID)
	if err != nil || got.Title != created.Title {
		t.Fatalf("got=%+v err=%v", got, err)
	}
}

func TestTaskSortingAndFilters(t *testing.T) {
	service, closeDB := testService(t)
	defer closeDB()
	ctx := context.Background()
	base := time.Date(2026, 7, 15, 10, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return base }
	first, _ := service.Create(ctx, CreateInput{Title: "first"})
	service.now = func() time.Time { return base.Add(time.Minute) }
	second, _ := service.Create(ctx, CreateInput{Title: "second"})
	low, high := -10, 10
	_, _ = service.Update(ctx, first.ID, UpdateInput{SortOrder: &high})
	_, _ = service.Update(ctx, second.ID, UpdateInput{SortOrder: &low})
	completed := StatusCompleted
	_, _ = service.Update(ctx, second.ID, UpdateInput{Status: &completed})
	result, err := service.List(ctx, ListOptions{Status: "all", Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Tasks) != 2 || result.Tasks[0].ID != first.ID || result.Total != 2 {
		t.Fatalf("result=%+v", result)
	}
	pending, err := service.List(ctx, ListOptions{Status: StatusPending, Limit: 1})
	if err != nil || pending.Total != 1 || len(pending.Tasks) != 1 {
		t.Fatalf("pending=%+v err=%v", pending, err)
	}
}
