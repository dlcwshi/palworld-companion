package auth

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dlcwshi/palworld-companion/internal/palworld"
	"github.com/dlcwshi/palworld-companion/internal/storage"
)

type playerClient struct {
	players palworld.Players
	err     error
}

func (p *playerClient) GetInfo(context.Context) (palworld.Info, error) { return palworld.Info{}, p.err }
func (p *playerClient) GetMetrics(context.Context) (palworld.Metrics, error) {
	return palworld.Metrics{}, p.err
}
func (p *playerClient) GetPlayers(context.Context) (palworld.Players, error) { return p.players, p.err }

type acceptVerifier struct {
	id  string
	err error
}

func (v acceptVerifier) Verify(context.Context, url.Values) (string, error) { return v.id, v.err }

func TestSafeReturnPathAndClaimedID(t *testing.T) {
	for _, bad := range []string{"//evil", "https://evil.test/", "javascript:alert(1)", "/ok\r\nX: y"} {
		if _, err := SafeReturnPath(bad); err == nil {
			t.Fatalf("accepted %q", bad)
		}
	}
	if got, err := SafeReturnPath("/tasks?scope=mine"); err != nil || got != "/tasks?scope=mine" {
		t.Fatalf("got=%q err=%v", got, err)
	}
	valid := "76561198000000000"
	if got, err := parseClaimedID("http://steamcommunity.com/openid/id/" + valid); err != nil || got != valid {
		t.Fatalf("got=%q err=%v", got, err)
	}
	for _, bad := range []string{"https://evil.test/openid/id/1", "https://steamcommunity.com/other/1", "https://steamcommunity.com/openid/id/nope", "https://steamcommunity.com/openid/id/18446744073709551616"} {
		if _, err := parseClaimedID(bad); err == nil {
			t.Fatalf("accepted %q", bad)
		}
	}
}

func TestSteamVerifierChecksProvider(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil || r.Form.Get("openid.mode") != "check_authentication" {
			t.Fatalf("form=%v err=%v", r.Form, err)
		}
		_, _ = w.Write([]byte("ns:http://specs.openid.net/auth/2.0\nis_valid:true\n"))
	}))
	defer server.Close()
	id := "76561198000000000"
	values := url.Values{"openid.claimed_id": {"https://steamcommunity.com/openid/id/" + id}, "openid.identity": {"https://steamcommunity.com/openid/id/" + id}}
	got, err := (SteamVerifier{Client: server.Client(), Endpoint: server.URL}).Verify(context.Background(), values)
	if err != nil || got != id {
		t.Fatalf("got=%q err=%v", got, err)
	}
}

func callbackValues(t *testing.T, location string) (url.Values, string) {
	t.Helper()
	provider, err := url.Parse(location)
	if err != nil {
		t.Fatal(err)
	}
	returnTo := provider.Query().Get("openid.return_to")
	callback, err := url.Parse(returnTo)
	if err != nil {
		t.Fatal(err)
	}
	state := callback.Query().Get("state")
	id := "https://steamcommunity.com/openid/id/76561198000000000"
	return url.Values{"state": {state}, "openid.mode": {"id_res"}, "openid.return_to": {returnTo}, "openid.claimed_id": {id}, "openid.identity": {id}}, state
}

func TestRegistrationSessionReplayAndExistingOfflineLogin(t *testing.T) {
	db, err := storage.Open(filepath.Join(t.TempDir(), "auth.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	client := &playerClient{players: palworld.Players{Players: []palworld.Player{{Name: "Builder", AccountName: "steam-account", UserID: "steam_76561198000000000", PlayerID: "player-1"}}}}
	service := NewService(NewRepository(db.SQL()), client, acceptVerifier{id: "76561198000000000"}, true, "https://pal.example/", time.Hour, nil)
	location, state, err := service.Begin(context.Background(), "/tasks")
	if err != nil {
		t.Fatal(err)
	}
	values, _ := callbackValues(t, location)
	user, token, path, err := service.Callback(context.Background(), values, state)
	if err != nil {
		t.Fatal(err)
	}
	if user.CharacterName != "Builder" || user.Role != RolePlayer || token == "" || path != "/tasks" {
		t.Fatalf("user=%+v token=%q path=%q", user, token, path)
	}
	var stored string
	if err := db.SQL().QueryRow(`SELECT token_hash FROM sessions`).Scan(&stored); err != nil || stored == token || stored != tokenHash(token) {
		t.Fatalf("stored=%q err=%v", stored, err)
	}
	if _, _, _, err := service.Callback(context.Background(), values, state); !errors.Is(err, ErrInvalidFlow) {
		t.Fatalf("replay err=%v", err)
	}
	client.err = errors.New("offline")
	location, state, err = service.Begin(context.Background(), "/tasks")
	if err != nil {
		t.Fatal(err)
	}
	values, _ = callbackValues(t, location)
	if _, _, _, err := service.Callback(context.Background(), values, state); err != nil {
		t.Fatalf("existing offline login: %v", err)
	}
}

func TestFirstRegistrationRequiresFreshOnlinePlayer(t *testing.T) {
	for _, tc := range []struct {
		name   string
		client *playerClient
		want   error
	}{{"offline", &playerClient{}, ErrPlayerOffline}, {"upstream", &playerClient{err: errors.New("down")}, ErrUpstream}} {
		t.Run(tc.name, func(t *testing.T) {
			db, _ := storage.Open(filepath.Join(t.TempDir(), "a.db"))
			defer db.Close()
			service := NewService(NewRepository(db.SQL()), tc.client, acceptVerifier{id: "76561198000000000"}, true, "https://pal.example/", time.Hour, nil)
			location, state, _ := service.Begin(context.Background(), "/tasks")
			values, _ := callbackValues(t, location)
			if _, _, _, err := service.Callback(context.Background(), values, state); !errors.Is(err, tc.want) {
				t.Fatalf("err=%v want=%v", err, tc.want)
			}
			var count int
			_ = db.SQL().QueryRow(`SELECT count(*) FROM users`).Scan(&count)
			if count != 0 {
				t.Fatalf("users=%d", count)
			}
		})
	}
}

func TestDisabledUserCannotReuseSession(t *testing.T) {
	db, _ := storage.Open(filepath.Join(t.TempDir(), "a.db"))
	defer db.Close()
	repo := NewRepository(db.SQL())
	now := time.Now().UTC()
	seen := now
	u, err := repo.CreateUser(context.Background(), User{SteamID: "1", PalworldUserID: "steam_1", CharacterName: "A", Role: RolePlayer, Status: StatusActive, CreatedAt: now, UpdatedAt: now, LastLoginAt: now, LastSeenAt: &seen})
	if err != nil {
		t.Fatal(err)
	}
	token := "secret"
	if err := repo.CreateSession(context.Background(), u.ID, tokenHash(token), now, now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := repo.SetStatus(context.Background(), 999, u.ID, StatusDisabled, now); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Authenticate(context.Background(), tokenHash(token), now); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("err=%v", err)
	}
}

func TestSteamVerifierRejectsInvalidProviderResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("is_valid:false")) }))
	defer server.Close()
	id := "https://steamcommunity.com/openid/id/76561198000000000"
	_, err := (SteamVerifier{Client: server.Client(), Endpoint: server.URL}).Verify(context.Background(), url.Values{"openid.claimed_id": {id}, "openid.identity": {id}})
	if err == nil || !strings.Contains(err.Error(), "rejected") {
		t.Fatalf("err=%v", err)
	}
}
