package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/dlcwshi/palworld-companion/internal/auth"
	"github.com/dlcwshi/palworld-companion/internal/palworld"
	"github.com/dlcwshi/palworld-companion/internal/roster"
	"github.com/dlcwshi/palworld-companion/internal/serverstatus"
	"github.com/dlcwshi/palworld-companion/internal/storage"
	"github.com/dlcwshi/palworld-companion/internal/tasks"
)

type mockClient struct {
	players palworld.Players
	err     error
}

func (m mockClient) GetInfo(context.Context) (palworld.Info, error) {
	version := "mock"
	return palworld.Info{ServerName: "Test", Version: &version}, m.err
}
func (m mockClient) GetMetrics(context.Context) (palworld.Metrics, error) {
	return palworld.Metrics{ServerFPS: 57, CurrentPlayers: 1, MaxPlayers: 50}, m.err
}
func (m mockClient) GetPlayers(context.Context) (palworld.Players, error) { return m.players, m.err }

type sequenceClient struct {
	players []palworld.Players
	calls   int
}

func (s *sequenceClient) GetInfo(context.Context) (palworld.Info, error) {
	return palworld.Info{}, nil
}
func (s *sequenceClient) GetMetrics(context.Context) (palworld.Metrics, error) {
	return palworld.Metrics{}, nil
}
func (s *sequenceClient) GetPlayers(context.Context) (palworld.Players, error) {
	value := s.players[s.calls]
	s.calls++
	return value, nil
}

type fixture struct {
	handler http.Handler
	db      *storage.DB
	logs    *bytes.Buffer
}

func newFixture(t *testing.T, client palworld.Client) *fixture {
	t.Helper()
	db, err := storage.Open(filepath.Join(t.TempDir(), "api.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	logs := &bytes.Buffer{}
	logger := slog.New(slog.NewJSONHandler(logs, nil))
	playerRoster := roster.NewService(roster.NewRepository(db.SQL()), client, time.Minute)
	status := serverstatus.New(client, playerRoster, time.Minute, time.Minute)
	service := auth.NewService(auth.NewRepository(db.SQL()), playerRoster, time.Hour)
	assets := fstest.MapFS{
		"index.html":             &fstest.MapFile{Data: []byte("<!doctype html><title>Companion</title>")},
		"sw.js":                  &fstest.MapFile{Data: []byte("self.skipWaiting()")},
		"manifest.webmanifest":   &fstest.MapFile{Data: []byte(`{"name":"Companion"}`)},
		"assets/app-deadbeef.js": &fstest.MapFile{Data: []byte("console.log('app')")},
	}
	return &fixture{handler: New(status, tasks.NewService(tasks.NewRepository(db.SQL())), service, BuildInfo{Name: "Palworld Companion", Version: "0.4.1-dev"}, logger, assets), db: db, logs: logs}
}
func jsonRequest(method, path, body string) *http.Request {
	return httptest.NewRequest(method, path, strings.NewReader(body))
}
func setupThroughHTTP(t *testing.T, f *fixture) (auth.User, *http.Cookie) {
	t.Helper()
	response := httptest.NewRecorder()
	f.handler.ServeHTTP(response, jsonRequest(http.MethodPost, "/api/v1/setup/admin", `{"username":"Owner","displayName":"Owner","password":"admin-password","confirmPassword":"admin-password"}`))
	if response.Code != http.StatusCreated {
		t.Fatalf("setup=%d %s", response.Code, response.Body.String())
	}
	var payload struct {
		User auth.User `json:"user"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	var cookie *http.Cookie
	for _, item := range response.Result().Cookies() {
		if item.Name == auth.SessionCookieName {
			cookie = item
		}
	}
	if cookie == nil {
		t.Fatal("missing session cookie")
	}
	return payload.User, cookie
}

func TestHealthPublicPlayersAndSPA(t *testing.T) {
	client := mockClient{players: palworld.Players{Players: []palworld.Player{{Name: "Safe", IP: "private", UserID: "steam_1", PlayerID: "internal", AccountName: "private-account"}}}}
	f := newFixture(t, client)
	for _, route := range []string{"/api/v1/health", "/api/v1/server/summary", "/api/v1/server/players"} {
		response := httptest.NewRecorder()
		f.handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, route, nil))
		if response.Code != http.StatusOK {
			t.Fatalf("%s=%d", route, response.Code)
		}
		if strings.Contains(response.Body.String(), "private") {
			t.Fatalf("sensitive player leak: %s", response.Body.String())
		}
		if route == "/api/v1/server/players" {
			body := response.Body.String()
			for _, required := range []string{`"currentStatusKnown":true`, `"status":"online"`, `"lastOnlineAt"`, `"counts"`} {
				if !strings.Contains(body, required) {
					t.Fatalf("players response missing %q: %s", required, body)
				}
			}
			for _, forbidden := range []string{"steam_1", "internal", "private-account", `"userId"`, `"playerId"`, `"accountName"`, `"ip"`} {
				if strings.Contains(body, forbidden) {
					t.Fatalf("players response leaked %q: %s", forbidden, body)
				}
			}
		}
	}
	response := httptest.NewRecorder()
	f.handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/account", nil))
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "Companion") {
		t.Fatalf("spa=%d", response.Code)
	}
}

func TestFrontendCacheHeaders(t *testing.T) {
	f := newFixture(t, mockClient{})
	for _, test := range []struct {
		path string
		want string
	}{
		{path: "/", want: "no-cache"},
		{path: "/account", want: "no-cache"},
		{path: "/sw.js", want: "no-cache"},
		{path: "/manifest.webmanifest", want: "no-cache"},
		{path: "/assets/app-deadbeef.js", want: "public, max-age=31536000, immutable"},
	} {
		response := httptest.NewRecorder()
		f.handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, test.path, nil))
		if response.Code != http.StatusOK || response.Header().Get("Cache-Control") != test.want {
			t.Errorf("%s: status=%d cache=%q", test.path, response.Code, response.Header().Get("Cache-Control"))
		}
	}
}

func TestSetupCreatesSecureSessionAndClosesPermanently(t *testing.T) {
	f := newFixture(t, mockClient{})
	status := httptest.NewRecorder()
	f.handler.ServeHTTP(status, httptest.NewRequest(http.MethodGet, "/api/v1/setup/status", nil))
	if status.Code != http.StatusOK || !strings.Contains(status.Body.String(), `"setupRequired":true`) {
		t.Fatalf("status=%d %s", status.Code, status.Body.String())
	}
	user, cookie := setupThroughHTTP(t, f)
	if user.Role != auth.RoleAdmin || user.SteamID != nil {
		t.Fatalf("user=%+v", user)
	}
	if !cookie.HttpOnly || !cookie.Secure || cookie.SameSite != http.SameSiteLaxMode || cookie.Path != "/" || cookie.MaxAge != 3600 {
		t.Fatalf("cookie=%+v", cookie)
	}
	var stored string
	if err := f.db.SQL().QueryRow(`SELECT token_hash FROM sessions`).Scan(&stored); err != nil || stored == cookie.Value {
		t.Fatalf("hash=%q err=%v", stored, err)
	}
	status = httptest.NewRecorder()
	f.handler.ServeHTTP(status, httptest.NewRequest(http.MethodGet, "/api/v1/setup/status", nil))
	if !strings.Contains(status.Body.String(), `"setupRequired":false`) {
		t.Fatal(status.Body.String())
	}
	second := httptest.NewRecorder()
	f.handler.ServeHTTP(second, jsonRequest(http.MethodPost, "/api/v1/setup/admin", `{"username":"Again","password":"another-password","confirmPassword":"another-password"}`))
	if second.Code != http.StatusConflict || !strings.Contains(second.Body.String(), "already_initialized") {
		t.Fatalf("second=%d %s", second.Code, second.Body.String())
	}
	if strings.Contains(f.logs.String(), "admin-password") || strings.Contains(f.logs.String(), cookie.Value) {
		t.Fatalf("secret leaked to log: %s", f.logs.String())
	}
}

func TestRegistrationApprovalLocalLoginAndTaskAuthentication(t *testing.T) {
	steamID := "76561198000000000"
	client := mockClient{players: palworld.Players{Players: []palworld.Player{{Name: "Builder", UserID: "steam_" + steamID, PlayerID: "player-1", AccountName: "account"}}}}
	f := newFixture(t, client)
	admin, adminCookie := setupThroughHTTP(t, f)
	register := httptest.NewRecorder()
	f.handler.ServeHTTP(register, jsonRequest(http.MethodPost, "/api/v1/auth/register", `{"characterName":"Builder","password":"player-password","confirmPassword":"player-password"}`))
	if register.Code != http.StatusCreated || !strings.Contains(register.Body.String(), "pending") {
		t.Fatalf("register=%d %s", register.Code, register.Body.String())
	}
	if body := register.Body.String(); strings.Contains(body, steamID) || strings.Contains(body, "steam_") || strings.Contains(body, "player-1") || strings.Contains(body, "account") {
		t.Fatalf("registration identity leak: %s", body)
	}
	invalidFields := httptest.NewRecorder()
	f.handler.ServeHTTP(invalidFields, jsonRequest(http.MethodPost, "/api/v1/auth/register", `{"steamId":"2","password":"password","confirmPassword":"password","role":"admin"}`))
	if invalidFields.Code != http.StatusBadRequest {
		t.Fatalf("privilege fields=%d", invalidFields.Code)
	}
	pending := httptest.NewRecorder()
	f.handler.ServeHTTP(pending, jsonRequest(http.MethodPost, "/api/v1/auth/login", `{"account":"Builder","password":"player-password"}`))
	if pending.Code != http.StatusForbidden || !strings.Contains(pending.Body.String(), "approval_pending") {
		t.Fatalf("pending=%d %s", pending.Code, pending.Body.String())
	}
	var playerID int64
	if err := f.db.SQL().QueryRow(`SELECT id FROM users WHERE steam_id=?`, steamID).Scan(&playerID); err != nil {
		t.Fatal(err)
	}
	approveReq := httptest.NewRequest(http.MethodPost, "/api/v1/admin/users/"+strconv64(playerID)+"/approve", nil)
	approveReq.AddCookie(adminCookie)
	approve := httptest.NewRecorder()
	f.handler.ServeHTTP(approve, approveReq)
	if approve.Code != http.StatusNoContent {
		t.Fatalf("approve=%d %s admin=%d", approve.Code, approve.Body.String(), admin.ID)
	}
	login := httptest.NewRecorder()
	f.handler.ServeHTTP(login, jsonRequest(http.MethodPost, "/api/v1/auth/login", `{"account":"Builder","password":"player-password"}`))
	if login.Code != http.StatusOK {
		t.Fatalf("login=%d %s", login.Code, login.Body.String())
	}
	playerCookie := login.Result().Cookies()[0]
	createReq := jsonRequest(http.MethodPost, "/api/v1/tasks", `{"title":"private","visibility":"personal"}`)
	createReq.AddCookie(playerCookie)
	created := httptest.NewRecorder()
	f.handler.ServeHTTP(created, createReq)
	if created.Code != http.StatusCreated {
		t.Fatalf("task=%d %s", created.Code, created.Body.String())
	}
	unauthorized := httptest.NewRecorder()
	f.handler.ServeHTTP(unauthorized, httptest.NewRequest(http.MethodGet, "/api/v1/tasks", nil))
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized=%d", unauthorized.Code)
	}
	playerAdminReq := httptest.NewRequest(http.MethodPost, "/api/v1/admin/users/"+strconv64(admin.ID)+"/approve", nil)
	playerAdminReq.AddCookie(playerCookie)
	forbidden := httptest.NewRecorder()
	f.handler.ServeHTTP(forbidden, playerAdminReq)
	if forbidden.Code != http.StatusForbidden {
		t.Fatalf("player admin=%d", forbidden.Code)
	}
}
func strconv64(value int64) string {
	const digits = "0123456789"
	if value == 0 {
		return "0"
	}
	buf := make([]byte, 0, 20)
	for value > 0 {
		buf = append([]byte{digits[value%10]}, buf...)
		value /= 10
	}
	return string(buf)
}

func TestSteamRoutesGoneWithoutProviderAccess(t *testing.T) {
	f := newFixture(t, mockClient{err: errors.New("must not be called")})
	for _, route := range []string{"/api/v1/auth/steam", "/api/v1/auth/steam/callback"} {
		response := httptest.NewRecorder()
		f.handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, route, nil))
		if response.Code != http.StatusGone || !strings.Contains(response.Body.String(), "steam_auth_disabled") {
			t.Fatalf("%s=%d %s", route, response.Code, response.Body.String())
		}
	}
}
func TestLoginRateLimit(t *testing.T) {
	f := newFixture(t, mockClient{})
	setupThroughHTTP(t, f)
	for i := 0; i < 11; i++ {
		response := httptest.NewRecorder()
		request := jsonRequest(http.MethodPost, "/api/v1/auth/login", `{"account":"missing","password":"wrong-password"}`)
		request.RemoteAddr = "192.0.2.10:1234"
		f.handler.ServeHTTP(response, request)
		if i < 10 && response.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d=%d", i, response.Code)
		}
		if i == 10 && response.Code != http.StatusTooManyRequests {
			t.Fatalf("rate limit=%d %s", response.Code, response.Body.String())
		}
	}
}

var _ io.Writer = (*bytes.Buffer)(nil)

func TestRegistrationRequestCompatibilityAndErrors(t *testing.T) {
	passwordBody := `"password":"player-password","confirmPassword":"player-password"`
	testError := func(name string, client mockClient, body, code string, status int) {
		t.Helper()
		t.Run(name, func(t *testing.T) {
			f := newFixture(t, client)
			setupThroughHTTP(t, f)
			response := httptest.NewRecorder()
			f.handler.ServeHTTP(response, jsonRequest(http.MethodPost, "/api/v1/auth/register", body))
			if response.Code != status || !strings.Contains(response.Body.String(), `"code":"`+code+`"`) {
				t.Fatalf("response=%d %s", response.Code, response.Body.String())
			}
			var count int
			if err := f.db.SQL().QueryRow(`SELECT count(*) FROM users WHERE role='player'`).Scan(&count); err != nil || count != 0 {
				t.Fatalf("players=%d err=%v", count, err)
			}
		})
	}

	testError("offline", mockClient{}, `{`+`"characterName":"Missing",`+passwordBody+`}`, "player_not_online", http.StatusConflict)
	testError("upstream", mockClient{err: errors.New("down")}, `{`+`"characterName":"Missing",`+passwordBody+`}`, "palworld_unavailable", http.StatusServiceUnavailable)
	testError("ambiguous", mockClient{players: palworld.Players{Players: []palworld.Player{{Name: "Twin", UserID: "steam_1"}, {Name: "Twin", UserID: "steam_2"}}}}, `{`+`"characterName":"Twin",`+passwordBody+`}`, "player_name_ambiguous", http.StatusConflict)
	testError("identity", mockClient{players: palworld.Players{Players: []palworld.Player{{Name: "Broken", UserID: "invalid"}}}}, `{`+`"characterName":"Broken",`+passwordBody+`}`, "player_identity_unavailable", http.StatusConflict)
	testError("both identifiers", mockClient{}, `{`+`"characterName":"Player","steamId":"1",`+passwordBody+`}`, "invalid_request", http.StatusBadRequest)
	testError("missing identifier", mockClient{}, `{`+passwordBody+`}`, "invalid_request", http.StatusBadRequest)

	steamID := "76561198000000030"
	client := mockClient{players: palworld.Players{Players: []palworld.Player{{Name: "Legacy", UserID: "steam_" + steamID, PlayerID: "legacy-player"}}}}
	legacy := newFixture(t, client)
	setupThroughHTTP(t, legacy)
	response := httptest.NewRecorder()
	legacy.handler.ServeHTTP(response, jsonRequest(http.MethodPost, "/api/v1/auth/register", `{`+`"steamId":"`+steamID+`",`+passwordBody+`}`))
	if response.Code != http.StatusCreated {
		t.Fatalf("legacy=%d %s", response.Code, response.Body.String())
	}
	duplicate := httptest.NewRecorder()
	legacy.handler.ServeHTTP(duplicate, jsonRequest(http.MethodPost, "/api/v1/auth/register", `{`+`"characterName":"Legacy",`+passwordBody+`}`))
	if duplicate.Code != http.StatusConflict || !strings.Contains(duplicate.Body.String(), `"code":"duplicate_account"`) {
		t.Fatalf("duplicate=%d %s", duplicate.Code, duplicate.Body.String())
	}
}

func TestRegistrationBypassesPlayerStatusCache(t *testing.T) {
	client := &sequenceClient{players: []palworld.Players{
		{Players: []palworld.Player{{Name: "Cached", UserID: "steam_76561198000000040"}}},
		{},
	}}
	f := newFixture(t, client)
	setupThroughHTTP(t, f)
	cached := httptest.NewRecorder()
	f.handler.ServeHTTP(cached, httptest.NewRequest(http.MethodGet, "/api/v1/server/players", nil))
	if cached.Code != http.StatusOK || !strings.Contains(cached.Body.String(), "Cached") {
		t.Fatalf("prime cache=%d %s", cached.Code, cached.Body.String())
	}
	response := httptest.NewRecorder()
	f.handler.ServeHTTP(response, jsonRequest(http.MethodPost, "/api/v1/auth/register", `{"characterName":"Cached","password":"player-password","confirmPassword":"player-password"}`))
	if response.Code != http.StatusConflict || !strings.Contains(response.Body.String(), `"code":"player_not_online"`) || client.calls != 2 {
		t.Fatalf("registration=%d calls=%d body=%s", response.Code, client.calls, response.Body.String())
	}
}
