package httpapi

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/dlcwshi/palworld-companion/internal/palworld"
	"github.com/dlcwshi/palworld-companion/internal/serverstatus"
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

func testHandler() http.Handler {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	status := serverstatus.New(mockClient{}, time.Minute, time.Minute, time.Minute)
	assets := fstest.MapFS{"index.html": &fstest.MapFile{Data: []byte("<!doctype html><title>Companion</title>")}}
	return New(status, BuildInfo{Name: "Palworld Companion", Version: "0.1.0"}, logger, assets)
}

func TestHealth(t *testing.T) {
	response := httptest.NewRecorder()
	testHandler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/health", nil))
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
		testHandler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, route, nil))
		if response.Code != http.StatusOK {
			t.Fatalf("%s status=%d", route, response.Code)
		}
		if strings.Contains(response.Body.String(), "private") {
			t.Fatalf("%s leaked sensitive field: %s", route, response.Body.String())
		}
	}
}
func TestSPAAndUnknownAPI(t *testing.T) {
	response := httptest.NewRecorder()
	testHandler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/settings", nil))
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "Companion") {
		t.Fatalf("SPA response=%d %q", response.Code, response.Body.String())
	}
	response = httptest.NewRecorder()
	testHandler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/missing", nil))
	if response.Code != http.StatusNotFound {
		t.Fatalf("unknown API status=%d", response.Code)
	}
}
