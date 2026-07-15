package auth

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dlcwshi/palworld-companion/internal/palworld"
	"github.com/dlcwshi/palworld-companion/internal/storage"
)

type playerClient struct {
	mu      sync.Mutex
	players palworld.Players
	err     error
	calls   int
}

func (p *playerClient) GetInfo(context.Context) (palworld.Info, error) { return palworld.Info{}, p.err }
func (p *playerClient) GetMetrics(context.Context) (palworld.Metrics, error) {
	return palworld.Metrics{}, p.err
}
func (p *playerClient) GetPlayers(context.Context) (palworld.Players, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.calls++
	return p.players, p.err
}
func (p *playerClient) set(players palworld.Players, err error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.players = players
	p.err = err
}
func (p *playerClient) callCount() int { p.mu.Lock(); defer p.mu.Unlock(); return p.calls }

func authFixture(t *testing.T, client palworld.Client) (*storage.DB, *Service) {
	t.Helper()
	db, err := storage.Open(filepath.Join(t.TempDir(), "auth.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db, NewService(NewRepository(db.SQL()), client, time.Hour)
}
func setupAdmin(t *testing.T, service *Service) (User, string) {
	t.Helper()
	user, token, err := service.SetupAdmin(context.Background(), "Owner", "Server Owner", "admin-password")
	if err != nil {
		t.Fatal(err)
	}
	return user, token
}

func TestPasswordArgon2id(t *testing.T) {
	first, err := HashPassword("correct horse")
	if err != nil {
		t.Fatal(err)
	}
	second, err := HashPassword("correct horse")
	if err != nil {
		t.Fatal(err)
	}
	if first == second || !strings.HasPrefix(first, "$argon2id$v=19$") {
		t.Fatalf("hash format or salt reuse: %q %q", first, second)
	}
	ok, err := VerifyPassword("correct horse", first)
	if err != nil || !ok {
		t.Fatalf("verify=%v err=%v", ok, err)
	}
	ok, err = VerifyPassword("wrong password", first)
	if err != nil || ok {
		t.Fatalf("wrong verify=%v err=%v", ok, err)
	}
	for _, password := range []string{"", "short", strings.Repeat("x", MaxPasswordBytes+1)} {
		if err := ValidatePassword(password); !errors.Is(err, ErrInvalidPassword) {
			t.Fatalf("password length accepted: %d", len(password))
		}
	}
}

func TestInitialSetupPermanentAndConcurrent(t *testing.T) {
	db, service := authFixture(t, &playerClient{})
	required, err := service.SetupRequired(context.Background())
	if err != nil || !required {
		t.Fatalf("required=%v err=%v", required, err)
	}
	admin, token := setupAdmin(t, service)
	if admin.Role != RoleAdmin || admin.Status != StatusActive || admin.Username == nil || *admin.Username != "Owner" || admin.SteamID != nil || token == "" {
		t.Fatalf("admin=%+v token=%q", admin, token)
	}
	var stored string
	if err := db.SQL().QueryRow(`SELECT token_hash FROM sessions WHERE user_id=?`, admin.ID).Scan(&stored); err != nil || stored == token || stored != tokenHash(token) {
		t.Fatalf("stored=%q err=%v", stored, err)
	}
	if required, _ = service.SetupRequired(context.Background()); required {
		t.Fatal("setup reopened")
	}
	if _, _, err := service.SetupAdmin(context.Background(), "Other", "", "other-password"); !errors.Is(err, ErrAlreadyInitialized) {
		t.Fatalf("second setup=%v", err)
	}
	if _, err := db.SQL().Exec(`UPDATE users SET status='disabled' WHERE id=?`, admin.ID); err != nil {
		t.Fatal(err)
	}
	if required, _ = service.SetupRequired(context.Background()); required {
		t.Fatal("setup reopened after administrator failure")
	}

	_, concurrent := authFixture(t, &playerClient{})
	start := make(chan struct{})
	errs := make(chan error, 2)
	var wg sync.WaitGroup
	for _, name := range []string{"AdminOne", "AdminTwo"} {
		wg.Add(1)
		go func(username string) {
			defer wg.Done()
			<-start
			_, _, err := concurrent.SetupAdmin(context.Background(), username, "", "concurrent-password")
			errs <- err
		}(name)
	}
	close(start)
	wg.Wait()
	close(errs)
	success, conflict := 0, 0
	for err := range errs {
		if err == nil {
			success++
		} else if errors.Is(err, ErrAlreadyInitialized) {
			conflict++
		} else {
			t.Fatalf("concurrent error=%v", err)
		}
	}
	if success != 1 || conflict != 1 {
		t.Fatalf("success=%d conflict=%d", success, conflict)
	}
}

func TestPlayerRegistrationApprovalLoginAndStates(t *testing.T) {
	steamID := "76561198000000000"
	client := &playerClient{players: palworld.Players{Players: []palworld.Player{{Name: "Builder", AccountName: "account", UserID: "steam_" + steamID, PlayerID: "player-1"}}}}
	db, service := authFixture(t, client)
	admin, _ := setupAdmin(t, service)
	ctx := context.Background()
	player, err := service.Register(ctx, RegistrationInput{SteamID: steamID, Password: "player-password"})
	if err != nil {
		t.Fatal(err)
	}
	if player.Status != StatusPending || player.Role != RolePlayer || player.SteamID == nil || *player.SteamID != steamID || player.PalworldUserID == nil || *player.PalworldUserID != "steam_"+steamID {
		t.Fatalf("player=%+v", player)
	}
	if _, _, err := service.Login(ctx, steamID, "player-password"); !errors.Is(err, ErrApprovalPending) {
		t.Fatalf("pending login=%v", err)
	}
	if err := service.Approve(ctx, admin.ID, player.ID); err != nil {
		t.Fatal(err)
	}
	calls := client.callCount()
	client.set(palworld.Players{}, errors.New("down"))
	logged, token, err := service.Login(ctx, steamID, "player-password")
	if err != nil || logged.Status != StatusActive || token == "" {
		t.Fatalf("login=%+v token=%q err=%v", logged, token, err)
	}
	if client.callCount() != calls {
		t.Fatal("active login contacted Palworld")
	}
	if _, _, err := service.Login(ctx, steamID, "wrong-password"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("wrong login=%v", err)
	}
	if _, err := service.Register(ctx, RegistrationInput{SteamID: steamID, Password: "another-password"}); !errors.Is(err, ErrUpstream) {
		t.Fatalf("registration did not require fresh upstream: %v", err)
	}
	if err := service.SetStatus(ctx, admin.ID, player.ID, StatusDisabled); err != nil {
		t.Fatal(err)
	}
	if _, _, err := service.Login(ctx, steamID, "player-password"); !errors.Is(err, ErrAccountDisabled) {
		t.Fatalf("disabled login=%v", err)
	}
	if err := service.SetStatus(ctx, admin.ID, player.ID, StatusActive); err != nil {
		t.Fatal(err)
	}
	if err := service.SetStatus(ctx, admin.ID, player.ID, StatusDeleted); err != nil {
		t.Fatal(err)
	}
	if _, _, err := service.Login(ctx, steamID, "player-password"); !errors.Is(err, ErrAccountDeleted) {
		t.Fatalf("deleted login=%v", err)
	}
	if err := service.Restore(ctx, player.ID); err != nil {
		t.Fatal(err)
	}
	var count int
	_ = db.SQL().QueryRow(`SELECT count(*) FROM users WHERE steam_id=?`, steamID).Scan(&count)
	if count != 1 {
		t.Fatalf("duplicate users=%d", count)
	}
}

func TestRegistrationOfflineUpstreamDuplicateAndRejected(t *testing.T) {
	client := &playerClient{}
	_, service := authFixture(t, client)
	admin, _ := setupAdmin(t, service)
	ctx := context.Background()
	steamID := "76561198000000001"
	if _, err := service.Register(ctx, RegistrationInput{SteamID: steamID, Password: "player-password"}); !errors.Is(err, ErrPlayerOffline) {
		t.Fatalf("offline=%v", err)
	}
	client.set(palworld.Players{}, errors.New("down"))
	if _, err := service.Register(ctx, RegistrationInput{SteamID: steamID, Password: "player-password"}); !errors.Is(err, ErrUpstream) {
		t.Fatalf("upstream=%v", err)
	}
	client.set(palworld.Players{Players: []palworld.Player{{Name: "P", UserID: "steam_" + steamID, PlayerID: "stable"}}}, nil)
	player, err := service.Register(ctx, RegistrationInput{SteamID: steamID, Password: "player-password"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Register(ctx, RegistrationInput{SteamID: steamID, Password: "player-password"}); !errors.Is(err, ErrDuplicateAccount) {
		t.Fatalf("duplicate=%v", err)
	}
	if err := service.Reject(ctx, admin.ID, player.ID, "not approved"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := service.Login(ctx, steamID, "player-password"); !errors.Is(err, ErrApplicationRejected) {
		t.Fatalf("rejected=%v", err)
	}
	if err := service.Approve(ctx, admin.ID, player.ID); err != nil {
		t.Fatal(err)
	}
	if _, _, err := service.Login(ctx, steamID, "player-password"); err != nil {
		t.Fatal(err)
	}
}

func TestAdminProtectionAndPasswordSessionRevocation(t *testing.T) {
	db, service := authFixture(t, &playerClient{})
	admin, currentToken := setupAdmin(t, service)
	ctx := context.Background()
	if err := service.SetStatus(ctx, admin.ID, admin.ID, StatusDisabled); !errors.Is(err, ErrUnsafeAdminAction) {
		t.Fatalf("self disable=%v", err)
	}
	if err := service.SetStatus(ctx, admin.ID, admin.ID, StatusDeleted); !errors.Is(err, ErrUnsafeAdminAction) {
		t.Fatalf("self delete=%v", err)
	}
	if err := service.SetRole(ctx, admin.ID, RolePlayer); !errors.Is(err, ErrUnsafeAdminAction) {
		t.Fatalf("last downgrade=%v", err)
	}
	otherToken := "other-session"
	now := time.Now().UTC()
	if err := NewRepository(db.SQL()).CreateSession(ctx, admin.ID, tokenHash(otherToken), now, now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	fresh, err := NewRepository(db.SQL()).FindByID(ctx, admin.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := service.ChangePassword(ctx, fresh, "admin-password", "new-admin-password", currentToken); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Authenticate(ctx, currentToken); err != nil {
		t.Fatalf("current session revoked: %v", err)
	}
	if _, err := service.Authenticate(ctx, otherToken); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("other session active: %v", err)
	}
	if _, _, err := service.Login(ctx, "OWNER", "new-admin-password"); err != nil {
		t.Fatalf("case insensitive username: %v", err)
	}
	if err := service.ResetPassword(ctx, admin.ID, "reset-password"); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Authenticate(ctx, currentToken); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("reset retained session: %v", err)
	}
}

func TestConcurrentRegistrationCreatesOneUser(t *testing.T) {
	steamID := "76561198000000002"
	client := &playerClient{players: palworld.Players{Players: []palworld.Player{{Name: "Concurrent", UserID: "steam_" + steamID, PlayerID: "concurrent-player"}}}}
	db, service := authFixture(t, client)
	setupAdmin(t, service)
	start := make(chan struct{})
	errs := make(chan error, 2)
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, err := service.Register(context.Background(), RegistrationInput{SteamID: steamID, Password: "player-password"})
			errs <- err
		}()
	}
	close(start)
	wg.Wait()
	close(errs)
	success, duplicate := 0, 0
	for err := range errs {
		if err == nil {
			success++
		} else if errors.Is(err, ErrDuplicateAccount) {
			duplicate++
		} else {
			t.Fatalf("registration=%v", err)
		}
	}
	var count int
	_ = db.SQL().QueryRow(`SELECT count(*) FROM users WHERE steam_id=?`, steamID).Scan(&count)
	if success != 1 || duplicate != 1 || count != 1 {
		t.Fatalf("success=%d duplicate=%d count=%d", success, duplicate, count)
	}
}

func TestCharacterNameRegistrationUsesFreshUniqueOnlineIdentity(t *testing.T) {
	steamID := "76561198000000010"
	client := &playerClient{players: palworld.Players{Players: []palworld.Player{{Name: "????", AccountName: "steam-account", UserID: "steam_" + steamID, PlayerID: "player-10"}}}}
	_, service := authFixture(t, client)
	setupAdmin(t, service)

	player, err := service.Register(context.Background(), RegistrationInput{CharacterName: "  ????  ", Password: "player-password"})
	if err != nil {
		t.Fatal(err)
	}
	if player.Status != StatusPending || player.Role != RolePlayer || player.SteamID == nil || *player.SteamID != steamID || player.PalworldUserID == nil || *player.PalworldUserID != "steam_"+steamID || player.PalworldPlayerID == nil || *player.PalworldPlayerID != "player-10" || player.CharacterName != "????" || player.AccountName != "steam-account" || player.LastSeenAt == nil {
		t.Fatalf("player=%+v", player)
	}
	if client.callCount() != 1 {
		t.Fatalf("fresh player calls=%d", client.callCount())
	}
}

func TestCharacterNameRegistrationFailures(t *testing.T) {
	client := &playerClient{}
	_, service := authFixture(t, client)
	setupAdmin(t, service)
	ctx := context.Background()
	password := "player-password"

	for _, input := range []RegistrationInput{
		{Password: password},
		{CharacterName: "Player", SteamID: "76561198000000011", Password: password},
		{CharacterName: "bad\nname", Password: password},
		{CharacterName: strings.Repeat("?", 81), Password: password},
	} {
		if _, err := service.Register(ctx, input); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("input=%+v err=%v", input, err)
		}
	}
	if _, err := service.Register(ctx, RegistrationInput{CharacterName: "Offline", Password: password}); !errors.Is(err, ErrPlayerOffline) {
		t.Fatalf("offline=%v", err)
	}
	client.set(palworld.Players{}, errors.New("down"))
	if _, err := service.Register(ctx, RegistrationInput{CharacterName: "Offline", Password: password}); !errors.Is(err, ErrUpstream) {
		t.Fatalf("upstream=%v", err)
	}
	client.set(palworld.Players{Players: []palworld.Player{{Name: "Twin", UserID: "steam_76561198000000012"}, {Name: "Twin", UserID: "steam_76561198000000013"}}}, nil)
	if _, err := service.Register(ctx, RegistrationInput{CharacterName: "Twin", Password: password}); !errors.Is(err, ErrPlayerNameAmbiguous) {
		t.Fatalf("ambiguous=%v", err)
	}
	for _, userID := range []string{"", "xbox_76561198000000012", "steam_", "steam_abc", "steam_0", "steam_18446744073709551616"} {
		client.set(palworld.Players{Players: []palworld.Player{{Name: "Broken", UserID: userID}}}, nil)
		if _, err := service.Register(ctx, RegistrationInput{CharacterName: "Broken", Password: password}); !errors.Is(err, ErrPlayerIdentity) {
			t.Fatalf("userID=%q err=%v", userID, err)
		}
	}
}

func TestCharacterNameLoginIsLocalAndRejectsAmbiguity(t *testing.T) {
	client := &playerClient{players: palworld.Players{Players: []palworld.Player{{Name: "Builder", UserID: "steam_76561198000000020", PlayerID: "player-20"}}}}
	db, service := authFixture(t, client)
	admin, _ := setupAdmin(t, service)
	ctx := context.Background()
	player, err := service.Register(ctx, RegistrationInput{CharacterName: "Builder", Password: "player-password"})
	if err != nil {
		t.Fatal(err)
	}
	if err := service.Approve(ctx, admin.ID, player.ID); err != nil {
		t.Fatal(err)
	}
	calls := client.callCount()
	client.set(palworld.Players{}, errors.New("down"))
	if _, _, err := service.Login(ctx, "Builder", "player-password"); err != nil {
		t.Fatalf("offline character login=%v", err)
	}
	if client.callCount() != calls {
		t.Fatal("character login contacted Palworld")
	}
	if _, _, err := service.Login(ctx, "Builder", "wrong-password"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("wrong password=%v", err)
	}

	hash, err := HashPassword("second-password")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	second, err := NewRepository(db.SQL()).CreatePendingPlayer(ctx, "76561198000000021", hash, "steam_76561198000000021", "player-21", "Builder", "other-account", now)
	if err != nil {
		t.Fatal(err)
	}
	if err := service.Approve(ctx, admin.ID, second.ID); err != nil {
		t.Fatal(err)
	}
	if _, _, err := service.Login(ctx, "Builder", "player-password"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("ambiguous character login=%v", err)
	}
	if _, _, err := service.Login(ctx, "76561198000000020", "player-password"); err != nil {
		t.Fatalf("SteamID fallback=%v", err)
	}
}
