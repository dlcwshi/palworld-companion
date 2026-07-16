package roster

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dlcwshi/palworld-companion/internal/palworld"
	"github.com/dlcwshi/palworld-companion/internal/storage"
)

type testClient struct {
	mu           sync.Mutex
	snapshot     palworld.Players
	err          error
	calls        int
	wait         chan struct{}
	started      chan struct{}
	beforeReturn func()
}

func (c *testClient) GetInfo(context.Context) (palworld.Info, error) { return palworld.Info{}, c.err }
func (c *testClient) GetMetrics(context.Context) (palworld.Metrics, error) {
	return palworld.Metrics{}, c.err
}
func (c *testClient) GetPlayers(context.Context) (palworld.Players, error) {
	c.mu.Lock()
	c.calls++
	snapshot, err, wait := c.snapshot, c.err, c.wait
	started, beforeReturn := c.started, c.beforeReturn
	c.mu.Unlock()
	if started != nil {
		select {
		case started <- struct{}{}:
		default:
		}
	}
	if wait != nil {
		<-wait
	}
	if beforeReturn != nil {
		beforeReturn()
	}
	return snapshot, err
}
func (c *testClient) set(snapshot palworld.Players, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.snapshot, c.err = snapshot, err
}
func (c *testClient) callCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.calls
}

func (c *testClient) configure(snapshot palworld.Players, err error, wait, started chan struct{}, beforeReturn func()) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.snapshot, c.err = snapshot, err
	c.wait, c.started, c.beforeReturn = wait, started, beforeReturn
}

type testClock struct {
	mu  sync.Mutex
	now time.Time
}

func newTestClock(now time.Time) *testClock { return &testClock{now: now} }
func (c *testClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}
func (c *testClock) Advance(duration time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(duration)
}
func rosterFixture(t *testing.T, client palworld.Client, ttl time.Duration) (*storage.DB, *Service) {
	t.Helper()
	db, err := storage.Open(filepath.Join(t.TempDir(), "roster.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db, NewService(NewRepository(db.SQL()), client, ttl)
}

func level(value int) *int { return &value }

func TestSynchronizationLifecycleAndBoundUserUpdates(t *testing.T) {
	ping, x, y := 42.5, 12.5, -4.5
	client := &testClient{snapshot: palworld.Players{Players: []palworld.Player{
		{Name: "Alpha", UserID: "steam_1", PlayerID: "player-1", AccountName: "account-1", Level: level(10)},
		{Name: "Beta", UserID: "steam_2", PlayerID: "player-2", AccountName: "account-2", Level: level(20), Ping: &ping, LocationX: &x, LocationY: &y},
	}}}
	db, service := rosterFixture(t, client, time.Second)
	now := time.Date(2026, 7, 16, 8, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return now }
	if _, err := db.SQL().Exec(`
INSERT INTO users(password_hash,steam_id,palworld_user_id,palworld_player_id,character_name,account_name,role,status,created_at,updated_at)
VALUES('hash','1','steam_1','old-player','Old Alpha','old-account','player','active',?,?)
`, timestamp(now.Add(-time.Hour)), timestamp(now.Add(-time.Hour))); err != nil {
		t.Fatal(err)
	}

	first := service.Players(context.Background())
	if !first.Available || !first.CurrentStatusKnown || first.Cached || first.Stale || first.Counts.Total != 2 || *first.Counts.CurrentOnline != 2 || first.Players[1].Ping == nil || first.Players[1].Position == nil {
		t.Fatalf("first=%+v", first)
	}
	var character, playerID, account, lastSeen string
	if err := db.SQL().QueryRow(`SELECT character_name,palworld_player_id,account_name,last_seen_at FROM users WHERE palworld_user_id='steam_1'`).Scan(&character, &playerID, &account, &lastSeen); err != nil {
		t.Fatal(err)
	}
	if character != "Alpha" || playerID != "player-1" || account != "account-1" || lastSeen != timestamp(now) {
		t.Fatalf("user=%q %q %q %q", character, playerID, account, lastSeen)
	}

	now = now.Add(2 * time.Second)
	client.set(palworld.Players{Players: []palworld.Player{
		{Name: "Alpha Renamed", UserID: "steam_1", PlayerID: "player-1-new", AccountName: "account-new", Level: level(55)},
	}}, nil)
	second := service.Players(context.Background())
	if second.Counts.Total != 2 || second.Counts.CurrentOnline == nil || *second.Counts.CurrentOnline != 1 || second.Counts.LastKnownOffline != 1 || len(second.Players) != 2 || second.Players[0].Name != "Alpha Renamed" || second.Players[0].Level != 55 || second.Players[1].Name != "Beta" || second.Players[1].Status != StatusOffline || second.Players[1].Ping != nil || second.Players[1].Position != nil {
		t.Fatalf("second=%+v", second)
	}
	betaLastOnline := second.Players[1].LastOnlineAt
	if !betaLastOnline.Equal(time.Date(2026, 7, 16, 8, 0, 0, 0, time.UTC)) {
		t.Fatalf("beta last online=%s", betaLastOnline)
	}

	now = now.Add(2 * time.Second)
	client.set(palworld.Players{Players: []palworld.Player{}}, nil)
	empty := service.Players(context.Background())
	if !empty.Available || *empty.Counts.CurrentOnline != 0 || empty.Counts.LastKnownOffline != 2 {
		t.Fatalf("empty=%+v", empty)
	}
	if !empty.Players[1].LastOnlineAt.Equal(betaLastOnline) {
		t.Fatal("offline player last_online_at changed")
	}

	now = now.Add(2 * time.Second)
	client.set(palworld.Players{Players: []palworld.Player{{Name: "Beta", UserID: "steam_2", PlayerID: "player-2", Level: level(21)}}}, nil)
	rejoined := service.Players(context.Background())
	if rejoined.Players[0].Name != "Beta" || rejoined.Players[0].Status != StatusOnline || !rejoined.Players[0].LastOnlineAt.Equal(now) {
		t.Fatalf("rejoined=%+v", rejoined)
	}
}

func TestFailuresCacheRestartAndEmptySynchronizationSemantics(t *testing.T) {
	client := &testClient{snapshot: palworld.Players{Players: []palworld.Player{{Name: "One", UserID: "steam_1"}}}}
	db, service := rosterFixture(t, client, time.Second)
	now := time.Date(2026, 7, 16, 9, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return now }
	first := service.Players(context.Background())
	firstOnline := first.Players[0].LastOnlineAt
	if client.callCount() != 1 {
		t.Fatalf("calls=%d", client.callCount())
	}

	now = now.Add(500 * time.Millisecond)
	cached := service.Players(context.Background())
	if !cached.Cached || cached.Stale || client.callCount() != 1 || !cached.Players[0].LastOnlineAt.Equal(firstOnline) {
		t.Fatalf("cached=%+v calls=%d", cached, client.callCount())
	}

	now = now.Add(time.Second)
	client.set(palworld.Players{}, errors.New("down"))
	failed := service.Players(context.Background())
	if failed.CurrentStatusKnown || !failed.Stale || failed.Players[0].Status != StatusUnknown || failed.Players[0].LastKnownStatus != StatusOnline || !failed.Players[0].LastOnlineAt.Equal(firstOnline) || failed.Players[0].Ping != nil || failed.Players[0].Position != nil {
		t.Fatalf("failed=%+v", failed)
	}
	var lastSuccess string
	if err := db.SQL().QueryRow(`SELECT value FROM system_settings WHERE key=?`, lastSuccessSetting).Scan(&lastSuccess); err != nil || lastSuccess != timestamp(firstOnline) {
		t.Fatalf("last success=%q err=%v", lastSuccess, err)
	}

	restarted := NewService(NewRepository(db.SQL()), client, time.Second)
	restarted.now = func() time.Time { return now }
	afterRestart := restarted.Players(context.Background())
	if !afterRestart.Available || afterRestart.CurrentStatusKnown || afterRestart.Players[0].Status != StatusUnknown {
		t.Fatalf("restart=%+v", afterRestart)
	}

	client.set(palworld.Players{Players: []palworld.Player{}}, nil)
	now = now.Add(time.Second)
	recovered := restarted.Players(context.Background())
	if !recovered.Available || !recovered.CurrentStatusKnown || recovered.UpdatedAt == nil || recovered.Counts.LastKnownOffline != 1 {
		t.Fatalf("recovered=%+v", recovered)
	}

	emptyDB, emptyService := rosterFixture(t, &testClient{err: errors.New("down")}, time.Second)
	never := emptyService.Players(context.Background())
	if never.Available || never.UpdatedAt != nil || never.CurrentStatusKnown {
		t.Fatalf("never=%+v", never)
	}
	_ = emptyDB
}

func TestTransactionFailureRollsBackEveryRosterField(t *testing.T) {
	client := &testClient{snapshot: palworld.Players{Players: []palworld.Player{{Name: "Owner", UserID: "steam_2", PlayerID: "taken"}}}}
	db, service := rosterFixture(t, client, 0)
	now := time.Date(2026, 7, 16, 10, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return now }
	if _, err := service.FreshPlayers(context.Background()); err != nil {
		t.Fatal(err)
	}
	client.set(palworld.Players{Players: []palworld.Player{{Name: "Intruder", UserID: "steam_1", PlayerID: "taken"}}}, nil)
	now = now.Add(time.Minute)
	if _, err := service.FreshPlayers(context.Background()); err == nil {
		t.Fatal("expected unique constraint failure")
	}
	state, lastSuccess, err := NewRepository(db.SQL()).State(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(state) != 1 || state[0].PalworldUserID != "steam_2" || !state[0].IsOnline || lastSuccess == nil || !lastSuccess.Equal(now.Add(-time.Minute)) {
		t.Fatalf("state=%+v last=%v", state, lastSuccess)
	}
}

func TestSlowFailureBackoffStartsAtCompletionAndRecoveryIsSingleflight(t *testing.T) {
	const callers = 4
	ttl := 3 * time.Second
	clock := newTestClock(time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC))
	releaseFailure := make(chan struct{})
	failureStarted := make(chan struct{}, 1)
	client := &testClient{}
	client.configure(palworld.Players{}, errors.New("slow failure"), releaseFailure, failureStarted, func() {
		clock.Advance(4 * time.Second)
	})
	db, service := rosterFixture(t, client, ttl)
	service.now = clock.Now

	start := make(chan struct{})
	results := make(chan Response, callers)
	var group sync.WaitGroup
	for i := 0; i < callers; i++ {
		group.Add(1)
		go func() {
			defer group.Done()
			<-start
			results <- service.Players(context.Background())
		}()
	}
	close(start)
	<-failureStarted
	close(releaseFailure)
	group.Wait()
	close(results)

	for result := range results {
		if result.CurrentStatusKnown || !result.Stale {
			t.Fatalf("slow failure result=%+v", result)
		}
	}
	if client.callCount() != 1 {
		t.Fatalf("slow failure upstream calls=%d", client.callCount())
	}

	clock.Advance(ttl - time.Nanosecond)
	withinCooldown := service.Players(context.Background())
	if withinCooldown.CurrentStatusKnown || !withinCooldown.Stale || client.callCount() != 1 {
		t.Fatalf("within cooldown=%+v calls=%d", withinCooldown, client.callCount())
	}

	releaseRecovery := make(chan struct{})
	recoveryStarted := make(chan struct{}, 1)
	client.configure(palworld.Players{Players: []palworld.Player{{Name: "Recovered", UserID: "steam_1"}}}, nil, releaseRecovery, recoveryStarted, nil)
	clock.Advance(2 * time.Nanosecond)

	start = make(chan struct{})
	results = make(chan Response, callers)
	group = sync.WaitGroup{}
	for i := 0; i < callers; i++ {
		group.Add(1)
		go func() {
			defer group.Done()
			<-start
			results <- service.Players(context.Background())
		}()
	}
	close(start)
	<-recoveryStarted
	close(releaseRecovery)
	group.Wait()
	close(results)

	for result := range results {
		if !result.CurrentStatusKnown || result.Stale || len(result.Players) != 1 || result.Players[0].Name != "Recovered" {
			t.Fatalf("recovery result=%+v", result)
		}
	}
	if client.callCount() != 2 {
		t.Fatalf("recovery upstream calls=%d", client.callCount())
	}
	if !service.failureAt.IsZero() || service.failureCode != "" {
		t.Fatalf("failure state not cleared: at=%v code=%q", service.failureAt, service.failureCode)
	}
	state, lastSuccess, err := NewRepository(db.SQL()).State(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(state) != 1 || lastSuccess == nil || !lastSuccess.Equal(clock.Now()) {
		t.Fatalf("state=%+v lastSuccess=%v now=%v", state, lastSuccess, clock.Now())
	}
}

func TestSlowFailurePreservesPersistentRosterState(t *testing.T) {
	ttl := 3 * time.Second
	clock := newTestClock(time.Date(2026, 7, 16, 13, 0, 0, 0, time.UTC))
	ping, x, y := 35.0, 10.5, -20.5
	client := &testClient{snapshot: palworld.Players{Players: []palworld.Player{{
		Name: "Known", UserID: "steam_1", Ping: &ping, LocationX: &x, LocationY: &y,
	}}}}
	db, service := rosterFixture(t, client, ttl)
	service.now = clock.Now
	first := service.Players(context.Background())
	if !first.CurrentStatusKnown || first.Players[0].Ping == nil || first.Players[0].Position == nil {
		t.Fatalf("first=%+v", first)
	}

	var beforeOnline, beforeCount int
	var beforeLastOnline, beforeLastSuccess string
	if err := db.SQL().QueryRow("SELECT is_online,last_online_at FROM player_roster WHERE palworld_user_id='steam_1'").Scan(&beforeOnline, &beforeLastOnline); err != nil {
		t.Fatal(err)
	}
	if err := db.SQL().QueryRow("SELECT count(*) FROM player_roster").Scan(&beforeCount); err != nil {
		t.Fatal(err)
	}
	if err := db.SQL().QueryRow("SELECT value FROM system_settings WHERE key=?", lastSuccessSetting).Scan(&beforeLastSuccess); err != nil {
		t.Fatal(err)
	}

	clock.Advance(ttl)
	client.configure(palworld.Players{}, errors.New("slow failure"), nil, nil, func() {
		clock.Advance(4 * time.Second)
	})
	failed := service.Players(context.Background())
	if failed.CurrentStatusKnown || !failed.Stale || len(failed.Players) != 1 || failed.Players[0].Status != StatusUnknown || failed.Players[0].Ping != nil || failed.Players[0].Position != nil {
		t.Fatalf("failed=%+v", failed)
	}

	var afterOnline, afterCount int
	var afterLastOnline, afterLastSuccess string
	if err := db.SQL().QueryRow("SELECT is_online,last_online_at FROM player_roster WHERE palworld_user_id='steam_1'").Scan(&afterOnline, &afterLastOnline); err != nil {
		t.Fatal(err)
	}
	if err := db.SQL().QueryRow("SELECT count(*) FROM player_roster").Scan(&afterCount); err != nil {
		t.Fatal(err)
	}
	if err := db.SQL().QueryRow("SELECT value FROM system_settings WHERE key=?", lastSuccessSetting).Scan(&afterLastSuccess); err != nil {
		t.Fatal(err)
	}
	if afterOnline != beforeOnline || afterLastOnline != beforeLastOnline || afterLastSuccess != beforeLastSuccess || afterCount != beforeCount {
		t.Fatalf("persistent state changed: before=%d %q %q %d after=%d %q %q %d", beforeOnline, beforeLastOnline, beforeLastSuccess, beforeCount, afterOnline, afterLastOnline, afterLastSuccess, afterCount)
	}

	clock.Advance(ttl - time.Nanosecond)
	cooled := service.Players(context.Background())
	if cooled.CurrentStatusKnown || !cooled.Stale || client.callCount() != 2 {
		t.Fatalf("cooled=%+v calls=%d", cooled, client.callCount())
	}
}

func TestFreshPlayersAlwaysCallsUpstreamAndClearsFailureOnSuccess(t *testing.T) {
	clock := newTestClock(time.Date(2026, 7, 16, 14, 0, 0, 0, time.UTC))
	client := &testClient{snapshot: palworld.Players{Players: []palworld.Player{{Name: "Cached", UserID: "steam_1"}}}}
	_, service := rosterFixture(t, client, time.Minute)
	service.now = clock.Now
	if result := service.Players(context.Background()); !result.CurrentStatusKnown {
		t.Fatalf("initial=%+v", result)
	}

	client.configure(palworld.Players{}, errors.New("fresh failure"), nil, nil, nil)
	for i := 0; i < 2; i++ {
		if _, err := service.FreshPlayers(context.Background()); err == nil || err.Error() != "fresh failure" {
			t.Fatalf("fresh failure %d: %v", i, err)
		}
	}
	if client.callCount() != 3 {
		t.Fatalf("fresh calls after failures=%d", client.callCount())
	}
	if service.failureAt.IsZero() || service.failureCode != PublicUpstreamError {
		t.Fatalf("fresh failure state: at=%v code=%q", service.failureAt, service.failureCode)
	}

	clock.Advance(time.Second)
	client.configure(palworld.Players{Players: []palworld.Player{{Name: "Fresh", UserID: "steam_2"}}}, nil, nil, nil, nil)
	snapshot, err := service.FreshPlayers(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if client.callCount() != 4 || len(snapshot.Players) != 1 || snapshot.Players[0].Name != "Fresh" {
		t.Fatalf("fresh snapshot=%+v calls=%d", snapshot, client.callCount())
	}
	if !service.failureAt.IsZero() || service.failureCode != "" {
		t.Fatalf("fresh success did not clear failure: at=%v code=%q", service.failureAt, service.failureCode)
	}
	cached := service.Players(context.Background())
	if !cached.Cached || !cached.CurrentStatusKnown || len(cached.Players) != 2 || client.callCount() != 4 {
		t.Fatalf("cached after fresh=%+v calls=%d", cached, client.callCount())
	}
}
func TestConcurrentRefreshSingleflightAndForcedFreshNoFallback(t *testing.T) {
	release := make(chan struct{})
	client := &testClient{
		snapshot: palworld.Players{Players: []palworld.Player{{Name: "One", UserID: "steam_1"}}},
		wait:     release,
	}
	_, service := rosterFixture(t, client, time.Minute)
	start := make(chan struct{})
	results := make(chan Response, 8)
	var group sync.WaitGroup
	for i := 0; i < 8; i++ {
		group.Add(1)
		go func() {
			defer group.Done()
			<-start
			results <- service.Players(context.Background())
		}()
	}
	close(start)
	for client.callCount() == 0 {
		time.Sleep(time.Millisecond)
	}
	close(release)
	group.Wait()
	close(results)
	for result := range results {
		if !result.CurrentStatusKnown || len(result.Players) != 1 {
			t.Fatalf("result=%+v", result)
		}
	}
	if client.callCount() != 1 {
		t.Fatalf("upstream calls=%d", client.callCount())
	}

	client.mu.Lock()
	client.wait = nil
	client.err = errors.New("down")
	client.mu.Unlock()
	if _, err := service.FreshPlayers(context.Background()); err == nil {
		t.Fatal("forced fresh used cache or stale fallback")
	}
}

func TestPublicJSONContainsNoInternalIdentity(t *testing.T) {
	client := &testClient{snapshot: palworld.Players{Players: []palworld.Player{{
		Name: "Safe", UserID: "steam_76561198000000001", PlayerID: "private-player",
		AccountName: "private-account", IP: "10.0.0.1",
	}}}}
	_, service := rosterFixture(t, client, time.Minute)
	response := service.Players(context.Background())
	body, err := json.Marshal(response)
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	for _, forbidden := range []string{"steam_76561198000000001", "private-player", "private-account", "10.0.0.1", "palworldUserId", "playerId", "accountName", `"id"`} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("public JSON leaked %q: %s", forbidden, text)
		}
	}
}
func TestInvalidSnapshotDoesNotModifyKnownPresence(t *testing.T) {
	client := &testClient{snapshot: palworld.Players{Players: []palworld.Player{{Name: "Safe", UserID: "steam_1"}}}}
	db, service := rosterFixture(t, client, 0)
	now := time.Date(2026, 7, 16, 11, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return now }
	first := service.Players(context.Background())
	if !first.CurrentStatusKnown {
		t.Fatalf("first=%+v", first)
	}

	now = now.Add(time.Minute)
	client.set(palworld.Players{Players: []palworld.Player{{Name: "Broken", UserID: ""}}}, nil)
	invalid := service.Players(context.Background())
	if invalid.CurrentStatusKnown || invalid.Players[0].Status != StatusUnknown || invalid.Players[0].LastKnownStatus != StatusOnline {
		t.Fatalf("invalid=%+v", invalid)
	}
	var online int
	var lastOnline, lastSuccess string
	if err := db.SQL().QueryRow(`SELECT is_online,last_online_at FROM player_roster WHERE palworld_user_id='steam_1'`).Scan(&online, &lastOnline); err != nil {
		t.Fatal(err)
	}
	if err := db.SQL().QueryRow(`SELECT value FROM system_settings WHERE key=?`, lastSuccessSetting).Scan(&lastSuccess); err != nil {
		t.Fatal(err)
	}
	if online != 1 || lastOnline != timestamp(now.Add(-time.Minute)) || lastSuccess != timestamp(now.Add(-time.Minute)) {
		t.Fatalf("online=%d lastOnline=%s lastSuccess=%s", online, lastOnline, lastSuccess)
	}
}
