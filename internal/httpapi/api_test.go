package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/dlcwshi/palworld-companion/internal/palworld"
	"github.com/dlcwshi/palworld-companion/internal/serverstatus"
	"github.com/dlcwshi/palworld-companion/internal/storage"
	"github.com/dlcwshi/palworld-companion/internal/tasks"
)

type mockClient struct{}

func (mockClient) GetInfo(context.Context) (palworld.Info, error) {
	version := "mock"
	return palworld.Info{ServerName: "Test", Version: &version}, nil
}
func (mockClient) GetMetrics(context.Context) (palworld.Metrics, error) {
	return palworld.Metrics{ServerFPS: 57, CurrentPlayers: 4, MaxPlayers: 50}, nil
}
func (mockClient) GetPlayers(context.Context) (palworld.Players, error) {
	return palworld.Players{Players: []palworld.Player{{Name: "Safe", IP: "private"}}}, nil
}

func testHandler(t *testing.T) http.Handler {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	status := serverstatus.New(mockClient{}, time.Minute, time.Minute, time.Minute)
	db, err := storage.Open(filepath.Join(t.TempDir(), "api.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	taskService := tasks.NewService(tasks.NewRepository(db.SQL()))
	assets := fstest.MapFS{"index.html": &fstest.MapFile{Data: []byte("<!doctype html><title>Companion</title>")}}
	return New(status, taskService, BuildInfo{Name: "Palworld Companion", Version: "0.1.0"}, logger, assets)
}

func TestHealth(t *testing.T) {
	response := httptest.NewRecorder()
	testHandler(t).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/health", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d", response.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["status"] != "ok" || body["version"] != "0.1.0" {
		t.Fatalf("body=%v", body)
	}
}
func TestSummaryAndPlayers(t *testing.T) {
	for _, route := range []string{"/api/v1/server/summary", "/api/v1/server/players"} {
		response := httptest.NewRecorder()
		testHandler(t).ServeHTTP(response, httptest.NewRequest(http.MethodGet, route, nil))
		if response.Code != http.StatusOK {
			t.Fatalf("%s status=%d", route, response.Code)
		}
		if strings.Contains(response.Body.String(), "private") {
			t.Fatalf("%s leaked sensitive field: %s", route, response.Body.String())
		}
	}
}
func TestSPAAndUnknownAPI(t *testing.T) {
	handler := testHandler(t)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/settings", nil))
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "Companion") {
		t.Fatalf("SPA response=%d %q", response.Code, response.Body.String())
	}
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/missing", nil))
	if response.Code != http.StatusNotFound {
		t.Fatalf("unknown API status=%d", response.Code)
	}
}
func TestTaskAPIWorkflow(t *testing.T) {
	handler := testHandler(t)
	create := httptest.NewRecorder()
	handler.ServeHTTP(create, httptest.NewRequest(http.MethodPost, "/api/v1/tasks", strings.NewReader(`{"title":"Tonight","notes":"prepare"}`)))
	if create.Code != http.StatusCreated {
		t.Fatalf("create=%d %s", create.Code, create.Body.String())
	}
	var task tasks.Task
	if err := json.Unmarshal(create.Body.Bytes(), &task); err != nil {
		t.Fatal(err)
	}
	list := httptest.NewRecorder()
	handler.ServeHTTP(list, httptest.NewRequest(http.MethodGet, "/api/v1/tasks?status=pending&limit=5", nil))
	if list.Code != http.StatusOK || !strings.Contains(list.Body.String(), `"total":1`) {
		t.Fatalf("list=%d %s", list.Code, list.Body.String())
	}
	patch := httptest.NewRecorder()
	handler.ServeHTTP(patch, httptest.NewRequest(http.MethodPatch, fmt.Sprintf("/api/v1/tasks/%d", task.ID), strings.NewReader(`{"status":"completed"}`)))
	if patch.Code != http.StatusOK || !strings.Contains(patch.Body.String(), `"status":"completed"`) {
		t.Fatalf("patch=%d %s", patch.Code, patch.Body.String())
	}
	remove := httptest.NewRecorder()
	handler.ServeHTTP(remove, httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/v1/tasks/%d", task.ID), nil))
	if remove.Code != http.StatusNoContent {
		t.Fatalf("delete=%d", remove.Code)
	}
}
func TestTaskAPIErrors(t *testing.T) {
	handler := testHandler(t)
	for _, test := range []struct {
		method, path, body string
		status             int
	}{{http.MethodPost, "/api/v1/tasks", `{"title":" "}`, http.StatusBadRequest}, {http.MethodPost, "/api/v1/tasks", `{bad`, http.StatusBadRequest}, {http.MethodGet, "/api/v1/tasks?status=invalid", "", http.StatusBadRequest}, {http.MethodGet, "/api/v1/tasks/999", "", http.StatusNotFound}} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(test.method, test.path, strings.NewReader(test.body)))
		if response.Code != test.status {
			t.Fatalf("%s %s=%d body=%s", test.method, test.path, response.Code, response.Body.String())
		}
	}
}
