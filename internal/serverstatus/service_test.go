package serverstatus

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dlcwshi/palworld-companion/internal/palworld"
	"github.com/dlcwshi/palworld-companion/internal/roster"
	"github.com/dlcwshi/palworld-companion/internal/storage"
)

type fakeClient struct {
	info    palworld.Info
	metrics palworld.Metrics
	players palworld.Players
	err     error
}

func (f *fakeClient) GetInfo(context.Context) (palworld.Info, error)       { return f.info, f.err }
func (f *fakeClient) GetMetrics(context.Context) (palworld.Metrics, error) { return f.metrics, f.err }
func (f *fakeClient) GetPlayers(context.Context) (palworld.Players, error) { return f.players, f.err }

func newStatusService(t *testing.T, client palworld.Client) *Service {
	t.Helper()
	db, err := storage.Open(filepath.Join(t.TempDir(), "status.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	playerRoster := roster.NewService(roster.NewRepository(db.SQL()), client, time.Minute)
	return New(client, playerRoster, time.Minute, time.Minute)
}

func TestSummaryNormalizesFields(t *testing.T) {
	version := "v1"
	client := &fakeClient{info: palworld.Info{ServerName: "Test", Version: &version}, metrics: palworld.Metrics{ServerFPS: 58.4, CurrentPlayers: 4, MaxPlayers: 50, Uptime: 60, Days: 3, BaseCampCount: 2}}
	result := newStatusService(t, client).Summary(context.Background())
	if !result.Available || result.Server == nil || result.Server.Name != "Test" || *result.Server.FPS != 58.4 || !result.Server.OnlinePlayersKnown || result.Server.OnlinePlayers == nil || *result.Server.OnlinePlayers != 0 || result.Error != nil {
		t.Fatalf("unexpected summary: %+v", result)
	}
}

func TestPlayersFilterSensitiveFields(t *testing.T) {
	level := 55
	x, y := 1.0, 2.0
	client := &fakeClient{players: palworld.Players{Players: []palworld.Player{{Name: "Safe", Level: &level, LocationX: &x, LocationY: &y, IP: "10.0.0.1", PlayerID: "secret-player", UserID: "steam_1"}}}}
	result := newStatusService(t, client).Players(context.Background())
	body, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	for _, secret := range []string{"10.0.0.1", "secret-player", "secret-user", "playerId", "userId", `"ip"`} {
		if strings.Contains(text, secret) {
			t.Fatalf("response leaked %q: %s", secret, text)
		}
	}
	if !result.Available || len(result.Players) != 1 || result.Players[0].Position == nil {
		t.Fatalf("unexpected players: %+v", result)
	}
}

func TestUnavailableDoesNotLeakError(t *testing.T) {
	client := &fakeClient{err: errors.New("dial 10.0.0.1 with password secret")}
	result := newStatusService(t, client).Summary(context.Background())
	if result.Available || result.Error == nil || *result.Error != publicUpstreamError {
		t.Fatalf("unexpected result: %+v", result)
	}
}
