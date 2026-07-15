package httpapi

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/dlcwshi/palworld-companion/internal/auth"
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
	return palworld.Players{Players: []palworld.Player{{Name: "Safe", IP: "private", UserID: "steam_76561198000000000", PlayerID: "internal", AccountName: "private-account"}}}, nil
}

type acceptOpenID struct{}

func (acceptOpenID) Verify(context.Context, url.Values) (string, error) {
	return "76561198000000000", nil
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
	return New(status, taskService, nil, BuildInfo{Name: "Palworld Companion", Version: "0.1.0"}, logger, assets)
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
	if create.Code != http.StatusServiceUnavailable {
		t.Fatalf("create=%d %s", create.Code, create.Body.String())
	}
	if create.Code != http.StatusCreated {
		return
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
	}{{http.MethodPost, "/api/v1/tasks", `{"title":" "}`, http.StatusServiceUnavailable}, {http.MethodPost, "/api/v1/tasks", `{bad`, http.StatusServiceUnavailable}, {http.MethodGet, "/api/v1/tasks?status=invalid", "", http.StatusServiceUnavailable}, {http.MethodGet, "/api/v1/tasks/999", "", http.StatusServiceUnavailable}} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(test.method, test.path, strings.NewReader(test.body)))
		if response.Code != test.status {
			t.Fatalf("%s %s=%d body=%s", test.method, test.path, response.Code, response.Body.String())
		}
	}
}

func TestAuthenticatedTaskAPIAndMe(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	client := mockClient{}
	status := serverstatus.New(client, time.Minute, time.Minute, time.Minute)
	db, err := storage.Open(filepath.Join(t.TempDir(), "authenticated.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	now := time.Now().UTC().Format(time.RFC3339Nano)
	result, err := db.SQL().Exec(`INSERT INTO users(steam_id,palworld_user_id,character_name,role,status,created_at,updated_at,last_login_at) VALUES('76561198000000000','steam_76561198000000000','Player A','player','active',?,?,?)`, now, now, now)
	if err != nil {
		t.Fatal(err)
	}
	userID, _ := result.LastInsertId()
	token := "test-session-token"
	sum := sha256.Sum256([]byte(token))
	hash := hex.EncodeToString(sum[:])
	if _, err := db.SQL().Exec(`INSERT INTO sessions(user_id,token_hash,created_at,expires_at,last_seen_at) VALUES(?,?,?,?,?)`, userID, hash, now, time.Now().Add(time.Hour).UTC().Format(time.RFC3339Nano), now); err != nil {
		t.Fatal(err)
	}
	authService := auth.NewService(auth.NewRepository(db.SQL()), client, auth.SteamVerifier{}, true, "https://pal.example/", time.Hour, nil)
	handler := New(status, tasks.NewService(tasks.NewRepository(db.SQL())), authService, BuildInfo{}, logger, fstest.MapFS{"index.html": &fstest.MapFile{Data: []byte("ok")}})
	cookie := &http.Cookie{Name: auth.SessionCookieName, Value: token}
	meReq := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	meReq.AddCookie(cookie)
	me := httptest.NewRecorder()
	handler.ServeHTTP(me, meReq)
	if me.Code != http.StatusOK || !strings.Contains(me.Body.String(), `"authenticated":true`) {
		t.Fatalf("me=%d %s", me.Code, me.Body.String())
	}
	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/tasks", strings.NewReader(`{"title":"private","notes":"","visibility":"personal"}`))
	createReq.AddCookie(cookie)
	created := httptest.NewRecorder()
	handler.ServeHTTP(created, createReq)
	if created.Code != http.StatusCreated || !strings.Contains(created.Body.String(), `"visibility":"personal"`) {
		t.Fatalf("created=%d %s", created.Code, created.Body.String())
	}
	unauthorized := httptest.NewRecorder()
	handler.ServeHTTP(unauthorized, httptest.NewRequest(http.MethodGet, "/api/v1/tasks", nil))
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized=%d", unauthorized.Code)
	}

}
func TestSteamRedirectCallbackAndSessionCookie(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	client := mockClient{}
	status := serverstatus.New(client, time.Minute, time.Minute, time.Minute)
	db, err := storage.Open(filepath.Join(t.TempDir(), "steam.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	authService := auth.NewService(auth.NewRepository(db.SQL()), client, acceptOpenID{}, true, "https://pal.example/", 2*time.Hour, nil)
	handler := New(status, tasks.NewService(tasks.NewRepository(db.SQL())), authService, BuildInfo{}, logger, fstest.MapFS{"index.html": &fstest.MapFile{Data: []byte("ok")}})
	missing := httptest.NewRecorder()
	handler.ServeHTTP(missing, httptest.NewRequest(http.MethodGet, "/api/v1/auth/steam/callback", nil))
	if missing.Code != http.StatusSeeOther {
		t.Fatalf("missing callback=%d", missing.Code)
	}
	var count int
	_ = db.SQL().QueryRow(`SELECT count(*) FROM users`).Scan(&count)
	if count != 0 {
		t.Fatalf("forged callback users=%d", count)
	}
	begin := httptest.NewRecorder()
	handler.ServeHTTP(begin, httptest.NewRequest(http.MethodGet, "/api/v1/auth/steam?returnTo=/tasks", nil))
	if begin.Code != http.StatusFound {
		t.Fatalf("begin=%d %s", begin.Code, begin.Body.String())
	}
	provider, err := url.Parse(begin.Header().Get("Location"))
	if err != nil || provider.Hostname() != "steamcommunity.com" {
		t.Fatalf("provider=%v err=%v", provider, err)
	}
	returnTo := provider.Query().Get("openid.return_to")
	callbackURL, _ := url.Parse(returnTo)
	state := callbackURL.Query().Get("state")
	var stateCookie *http.Cookie
	for _, cookie := range begin.Result().Cookies() {
		if cookie.Name == auth.StateCookieName {
			stateCookie = cookie
		}
	}
	if stateCookie == nil || !stateCookie.HttpOnly || !stateCookie.Secure || stateCookie.SameSite != http.SameSiteLaxMode || stateCookie.Path != "/api/v1/auth/steam/callback" {
		t.Fatalf("state cookie=%+v", stateCookie)
	}
	claimed := "https://steamcommunity.com/openid/id/76561198000000000"
	values := url.Values{"state": {state}, "openid.mode": {"id_res"}, "openid.return_to": {returnTo}, "openid.claimed_id": {claimed}, "openid.identity": {claimed}}
	request := httptest.NewRequest(http.MethodGet, "/api/v1/auth/steam/callback?"+values.Encode(), nil)
	request.AddCookie(stateCookie)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusSeeOther || response.Header().Get("Location") != "/tasks" {
		t.Fatalf("callback=%d location=%s body=%s", response.Code, response.Header().Get("Location"), response.Body.String())
	}
	var sessionCookie *http.Cookie
	for _, cookie := range response.Result().Cookies() {
		if cookie.Name == auth.SessionCookieName {
			sessionCookie = cookie
		}
	}
	if sessionCookie == nil || !sessionCookie.HttpOnly || !sessionCookie.Secure || sessionCookie.SameSite != http.SameSiteLaxMode || sessionCookie.Path != "/" || sessionCookie.MaxAge != 7200 {
		t.Fatalf("session cookie=%+v", sessionCookie)
	}
	var hash string
	if err := db.SQL().QueryRow(`SELECT token_hash FROM sessions`).Scan(&hash); err != nil || hash == sessionCookie.Value {
		t.Fatalf("hash=%q err=%v", hash, err)
	}
}
