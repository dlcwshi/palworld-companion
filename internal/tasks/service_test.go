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

func TestMultiPlayerTaskPermissions(t *testing.T) {
	db, err := storage.Open(filepath.Join(t.TempDir(), "permissions.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	for _, u := range []struct {
		id                int
		steam, name, role string
	}{{1, "1", "Player A", "player"}, {2, "2", "Player B", "player"}, {3, "3", "Admin", "admin"}} {
		_, err := db.SQL().Exec(`INSERT INTO users(id,steam_id,palworld_user_id,character_name,role,status,created_at,updated_at,last_login_at) VALUES(?,?,?,?,?,'active',?,?,?)`, u.id, u.steam, "steam_"+u.steam, u.name, u.role, now, now, now)
		if err != nil {
			t.Fatal(err)
		}
	}
	service := NewService(NewRepository(db.SQL()))
	ctx := context.Background()
	a := Actor{ID: 1, Role: "player"}
	b := Actor{ID: 2, Role: "player"}
	admin := Actor{ID: 3, Role: "admin"}
	personal, err := service.CreateFor(ctx, a, CreateInput{Title: "A private", Visibility: VisibilityPersonal})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.GetFor(ctx, b, personal.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("B read A personal: %v", err)
	}
	changed := "stolen"
	if _, err := service.UpdateFor(ctx, b, personal.ID, UpdateInput{Title: &changed}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("B edited A personal: %v", err)
	}
	if err := service.DeleteFor(ctx, b, personal.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("B deleted A personal: %v", err)
	}
	shared, err := service.CreateFor(ctx, a, CreateInput{Title: "A shared", Visibility: VisibilityShared})
	if err != nil {
		t.Fatal(err)
	}
	visible, err := service.GetFor(ctx, b, shared.ID)
	if err != nil || visible.CanManage {
		t.Fatalf("B shared=%+v err=%v", visible, err)
	}
	if _, err := service.UpdateFor(ctx, b, shared.ID, UpdateInput{Title: &changed}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("B edited A shared: %v", err)
	}
	if _, err := service.UpdateFor(ctx, a, shared.ID, UpdateInput{Title: &changed}); err != nil {
		t.Fatalf("A edit shared: %v", err)
	}
	if err := service.DeleteFor(ctx, admin, personal.ID); err != nil {
		t.Fatalf("admin delete: %v", err)
	}
	list, err := service.ListFor(ctx, b, ListOptions{Status: "all", Scope: "visible", Limit: 100})
	if err != nil || len(list.Tasks) != 1 || list.Tasks[0].ID != shared.ID {
		t.Fatalf("B list=%+v err=%v", list, err)
	}
}
