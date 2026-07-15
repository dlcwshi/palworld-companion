package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/dlcwshi/palworld-companion/internal/palworld"
)

type Service struct {
	repo      *Repository
	players   palworld.Client
	ttl       time.Duration
	dummyHash string
	now       func() time.Time
}

func NewService(repo *Repository, players palworld.Client, ttl time.Duration) *Service {
	dummy, _ := HashPassword("constant-time-dummy-password")
	return &Service{repo: repo, players: players, ttl: ttl, dummyHash: dummy, now: time.Now}
}
func (s *Service) SessionTTL() time.Duration { return s.ttl }
func randomToken() (string, error) {
	value := make([]byte, 32)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}
func tokenHash(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func validUsername(username string) bool {
	if username != strings.TrimSpace(username) || utf8.RuneCountInString(username) < 3 || utf8.RuneCountInString(username) > 64 {
		return false
	}
	for _, r := range username {
		if !unicode.IsLetter(r) && !unicode.IsNumber(r) && !strings.ContainsRune("_.-", r) {
			return false
		}
	}
	return true
}
func isSteamID(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	n, err := strconv.ParseUint(value, 10, 64)
	return err == nil && n > 0
}
func validateDisplayName(value string) error {
	if utf8.RuneCountInString(value) > 80 {
		return fmt.Errorf("%w: display name is too long", ErrInvalidInput)
	}
	return nil
}

func (s *Service) SetupRequired(ctx context.Context) (bool, error) { return s.repo.SetupRequired(ctx) }
func (s *Service) SetupAdmin(ctx context.Context, username, displayName, password string) (User, string, error) {
	if !validUsername(username) {
		return User{}, "", fmt.Errorf("%w: username must be 3-64 letters, numbers, dots, dashes, or underscores", ErrInvalidInput)
	}
	if err := validateDisplayName(displayName); err != nil {
		return User{}, "", err
	}
	hash, err := HashPassword(password)
	if err != nil {
		return User{}, "", err
	}
	token, err := randomToken()
	if err != nil {
		return User{}, "", err
	}
	now := s.now().UTC()
	user, err := s.repo.CreateInitialAdmin(ctx, username, strings.TrimSpace(displayName), hash, tokenHash(token), now, now.Add(s.ttl))
	if err != nil {
		return User{}, "", err
	}
	return user, token, nil
}
func (s *Service) CreateRecoveryAdmin(ctx context.Context, username, displayName, password string) (User, error) {
	if !validUsername(username) {
		return User{}, fmt.Errorf("%w: invalid username", ErrInvalidInput)
	}
	if err := validateDisplayName(displayName); err != nil {
		return User{}, err
	}
	hash, err := HashPassword(password)
	if err != nil {
		return User{}, err
	}
	return s.repo.CreateRecoveryAdmin(ctx, username, strings.TrimSpace(displayName), hash, s.now().UTC())
}

func (s *Service) Register(ctx context.Context, steamID, password string) (User, error) {
	required, err := s.repo.SetupRequired(ctx)
	if err != nil {
		return User{}, err
	}
	if required {
		return User{}, ErrSetupRequired
	}
	if !isSteamID(steamID) {
		return User{}, fmt.Errorf("%w: SteamID64 must contain decimal digits and fit uint64", ErrInvalidInput)
	}
	if err := ValidatePassword(password); err != nil {
		return User{}, err
	}
	players, err := palworld.GetPlayersFreshForIdentityBinding(ctx, s.players)
	if err != nil {
		return User{}, ErrUpstream
	}
	var match *palworld.Player
	for i := range players.Players {
		if players.Players[i].UserID == "steam_"+steamID {
			match = &players.Players[i]
			break
		}
	}
	if match == nil {
		return User{}, ErrPlayerOffline
	}
	hash, err := HashPassword(password)
	if err != nil {
		return User{}, err
	}
	return s.repo.CreatePendingPlayer(ctx, steamID, hash, match.UserID, match.PlayerID, match.Name, match.AccountName, s.now().UTC())
}

func (s *Service) Login(ctx context.Context, identifier, password string) (User, string, error) {
	required, err := s.repo.SetupRequired(ctx)
	if err != nil {
		return User{}, "", err
	}
	if required {
		return User{}, "", ErrSetupRequired
	}
	identifier = strings.TrimSpace(identifier)
	user, findErr := s.repo.ResolveByIdentifier(ctx, identifier)
	hash := s.dummyHash
	if findErr == nil && user.PasswordHash != nil {
		hash = *user.PasswordHash
	}
	valid, verifyErr := VerifyPassword(password, hash)
	if findErr != nil || verifyErr != nil || !valid {
		return User{}, "", ErrInvalidCredentials
	}
	switch user.Status {
	case StatusPending:
		return User{}, "", ErrApprovalPending
	case StatusDisabled:
		return User{}, "", ErrAccountDisabled
	case StatusRejected:
		return User{}, "", ErrApplicationRejected
	case StatusDeleted:
		return User{}, "", ErrAccountDeleted
	case StatusActive:
	default:
		return User{}, "", ErrInvalidCredentials
	}
	token, err := randomToken()
	if err != nil {
		return User{}, "", err
	}
	now := s.now().UTC()
	if err := s.repo.CreateSession(ctx, user.ID, tokenHash(token), now, now.Add(s.ttl)); err != nil {
		return User{}, "", err
	}
	if err := s.repo.UpdateLogin(ctx, user.ID, now); err != nil {
		return User{}, "", err
	}
	user, err = s.repo.FindByID(ctx, user.ID)
	return user, token, err
}

func (s *Service) Authenticate(ctx context.Context, token string) (User, error) {
	if token == "" {
		return User{}, ErrUnauthenticated
	}
	return s.repo.Authenticate(ctx, tokenHash(token), s.now().UTC())
}
func (s *Service) Logout(ctx context.Context, token string) error {
	if token == "" {
		return nil
	}
	return s.repo.RevokeToken(ctx, tokenHash(token), s.now().UTC())
}
func (s *Service) ChangePassword(ctx context.Context, user User, currentPassword, newPassword, currentToken string) error {
	if user.PasswordHash == nil {
		return ErrInvalidCredentials
	}
	valid, err := VerifyPassword(currentPassword, *user.PasswordHash)
	if err != nil || !valid {
		return ErrInvalidCredentials
	}
	hash, err := HashPassword(newPassword)
	if err != nil {
		return err
	}
	currentHash := tokenHash(currentToken)
	return s.repo.UpdatePassword(ctx, user.ID, hash, &currentHash, s.now().UTC())
}
func (s *Service) ResetPassword(ctx context.Context, userID int64, password string) error {
	hash, err := HashPassword(password)
	if err != nil {
		return err
	}
	return s.repo.UpdatePassword(ctx, userID, hash, nil, s.now().UTC())
}
func (s *Service) ResetPasswordByIdentifier(ctx context.Context, steamID, username, password string) error {
	var user User
	var err error
	if steamID != "" {
		user, err = s.repo.FindBySteamID(ctx, steamID)
	} else if username != "" {
		user, err = s.repo.FindByUsername(ctx, username)
	} else {
		return fmt.Errorf("--steam-id or --username is required")
	}
	if err != nil {
		return err
	}
	return s.ResetPassword(ctx, user.ID, password)
}

func validStatus(status string) bool {
	return status == "" || status == StatusPending || status == StatusActive || status == StatusDisabled || status == StatusRejected || status == StatusDeleted
}
func (s *Service) ListUsers(ctx context.Context, status string) ([]User, error) {
	if !validStatus(status) {
		return nil, fmt.Errorf("%w: invalid status filter", ErrInvalidInput)
	}
	return s.repo.ListUsers(ctx, status)
}
func (s *Service) SetStatus(ctx context.Context, current, target int64, status string) error {
	return s.repo.SetStatus(ctx, current, target, status, s.now().UTC())
}
func (s *Service) Restore(ctx context.Context, target int64) error {
	return s.repo.Restore(ctx, target, s.now().UTC())
}
func (s *Service) Approve(ctx context.Context, actor, target int64) error {
	return s.repo.Approve(ctx, target, &actor, s.now().UTC())
}
func (s *Service) ApproveBySteamID(ctx context.Context, steamID string) error {
	user, err := s.repo.FindBySteamID(ctx, steamID)
	if err != nil {
		return err
	}
	return s.repo.Approve(ctx, user.ID, nil, s.now().UTC())
}
func (s *Service) Reject(ctx context.Context, actor, target int64, reason string) error {
	if len(reason) > 500 {
		return fmt.Errorf("%w: rejection reason is too long", ErrInvalidInput)
	}
	return s.repo.Reject(ctx, target, &actor, reason, s.now().UTC())
}
func (s *Service) RejectBySteamID(ctx context.Context, steamID, reason string) error {
	user, err := s.repo.FindBySteamID(ctx, steamID)
	if err != nil {
		return err
	}
	return s.repo.Reject(ctx, user.ID, nil, reason, s.now().UTC())
}
func (s *Service) SetRole(ctx context.Context, target int64, role string) error {
	return s.repo.SetRole(ctx, target, role, s.now().UTC())
}
func (s *Service) SetRoleBySteamID(ctx context.Context, steamID, role string) error {
	return s.repo.SetRoleBySteamID(ctx, steamID, role, s.now().UTC())
}
func (s *Service) RevokeUserSessions(ctx context.Context, id int64) error {
	return s.repo.RevokeUserSessions(ctx, id, s.now().UTC())
}
func (s *Service) Cleanup(ctx context.Context) { s.repo.Cleanup(ctx, s.now().UTC()) }

func IsExpected(err error) bool {
	return errors.Is(err, ErrAlreadyInitialized) || errors.Is(err, ErrSetupRequired) || errors.Is(err, ErrInvalidCredentials) || errors.Is(err, ErrInvalidInput) || errors.Is(err, ErrApprovalPending) || errors.Is(err, ErrAccountDisabled) || errors.Is(err, ErrApplicationRejected) || errors.Is(err, ErrAccountDeleted) || errors.Is(err, ErrPlayerOffline) || errors.Is(err, ErrUpstream) || errors.Is(err, ErrDuplicateAccount) || errors.Is(err, ErrNotFound) || errors.Is(err, ErrInvalidTransition) || errors.Is(err, ErrUnsafeAdminAction) || errors.Is(err, ErrInvalidPassword)
}
