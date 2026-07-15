package httpapi

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/dlcwshi/palworld-companion/internal/auth"
)

type passwordConfirmation struct {
	Password        string `json:"password"`
	ConfirmPassword string `json:"confirmPassword"`
}
type setupRequest struct {
	Username        string `json:"username"`
	DisplayName     string `json:"displayName"`
	Password        string `json:"password"`
	ConfirmPassword string `json:"confirmPassword"`
}
type registerRequest struct {
	CharacterName   string `json:"characterName"`
	SteamID         string `json:"steamId"`
	Password        string `json:"password"`
	ConfirmPassword string `json:"confirmPassword"`
}
type loginRequest struct {
	Account  string `json:"account"`
	Password string `json:"password"`
}
type changePasswordRequest struct {
	CurrentPassword string `json:"currentPassword"`
	NewPassword     string `json:"newPassword"`
	ConfirmPassword string `json:"confirmPassword"`
}
type rejectRequest struct {
	Reason string `json:"reason"`
}
type roleRequest struct {
	Role string `json:"role"`
}

func (a *API) setupStatus(w http.ResponseWriter, r *http.Request) {
	if a.auth == nil {
		writeAPIError(w, http.StatusServiceUnavailable, "auth_unavailable", "Authentication service is unavailable.")
		return
	}
	required, err := a.auth.SetupRequired(r.Context())
	if err != nil {
		a.internal(w, "read setup status", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"setupRequired": required})
}
func (a *API) setupAdmin(w http.ResponseWriter, r *http.Request) {
	if !a.allowSensitive(w, r, "setup", 5) {
		return
	}
	var request setupRequest
	if decodeJSON(w, r, &request) != nil {
		return
	}
	if request.Password != request.ConfirmPassword {
		writeAPIError(w, http.StatusBadRequest, "password_mismatch", "Password confirmation does not match.")
		return
	}
	user, token, err := a.auth.SetupAdmin(r.Context(), request.Username, request.DisplayName, request.Password)
	if err != nil {
		a.authError(w, "initialize administrator", err)
		return
	}
	a.setSessionCookie(w, token)
	writeJSON(w, http.StatusCreated, map[string]any{"authenticated": true, "user": user})
}
func (a *API) steamDisabled(w http.ResponseWriter, _ *http.Request) {
	writeAPIError(w, http.StatusGone, "steam_auth_disabled", "Steam OpenID login is no longer enabled.")
}
func (a *API) register(w http.ResponseWriter, r *http.Request) {
	if !a.allowSensitive(w, r, "register", 5) {
		return
	}
	var request registerRequest
	if decodeJSON(w, r, &request) != nil {
		return
	}
	if request.Password != request.ConfirmPassword {
		writeAPIError(w, http.StatusBadRequest, "password_mismatch", "Password confirmation does not match.")
		return
	}
	user, err := a.auth.Register(r.Context(), auth.RegistrationInput{CharacterName: request.CharacterName, SteamID: request.SteamID, Password: request.Password})
	if err != nil {
		a.authError(w, "register player", err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"status": user.Status, "message": "Registration submitted for administrator approval.", "characterName": user.CharacterName})
}
func (a *API) login(w http.ResponseWriter, r *http.Request) {
	if !a.allowSensitive(w, r, "login", 10) {
		return
	}
	var request loginRequest
	if decodeJSON(w, r, &request) != nil {
		return
	}
	user, token, err := a.auth.Login(r.Context(), request.Account, request.Password)
	if err != nil {
		a.authError(w, "login", err)
		return
	}
	a.setSessionCookie(w, token)
	writeJSON(w, http.StatusOK, map[string]any{"authenticated": true, "user": user})
}
func (a *API) setSessionCookie(w http.ResponseWriter, token string) {
	ttl := a.auth.SessionTTL()
	http.SetCookie(w, &http.Cookie{Name: auth.SessionCookieName, Value: token, Path: "/", Expires: time.Now().Add(ttl), MaxAge: int(ttl.Seconds()), HttpOnly: true, Secure: true, SameSite: http.SameSiteLaxMode})
}
func (a *API) currentUser(r *http.Request) (auth.User, error) {
	if a.auth == nil {
		return auth.User{}, auth.ErrUnauthenticated
	}
	cookie, err := r.Cookie(auth.SessionCookieName)
	if err != nil {
		return auth.User{}, auth.ErrUnauthenticated
	}
	return a.auth.Authenticate(r.Context(), cookie.Value)
}
func (a *API) requireUser(w http.ResponseWriter, r *http.Request) (auth.User, bool) {
	user, err := a.currentUser(r)
	if err != nil {
		writeAPIError(w, http.StatusUnauthorized, "unauthenticated", "Authentication is required.")
		return auth.User{}, false
	}
	return user, true
}
func (a *API) me(w http.ResponseWriter, r *http.Request) {
	user, err := a.currentUser(r)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"authenticated": false, "user": nil})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"authenticated": true, "user": user})
}
func (a *API) logout(w http.ResponseWriter, r *http.Request) {
	cookie, _ := r.Cookie(auth.SessionCookieName)
	if cookie != nil && a.auth != nil {
		_ = a.auth.Logout(r.Context(), cookie.Value)
	}
	http.SetCookie(w, &http.Cookie{Name: auth.SessionCookieName, Value: "", Path: "/", MaxAge: -1, HttpOnly: true, Secure: true, SameSite: http.SameSiteLaxMode})
	w.WriteHeader(http.StatusNoContent)
}
func (a *API) changePassword(w http.ResponseWriter, r *http.Request) {
	if !a.allowSensitive(w, r, "change-password", 5) {
		return
	}
	user, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	var request changePasswordRequest
	if decodeJSON(w, r, &request) != nil {
		return
	}
	if request.NewPassword != request.ConfirmPassword {
		writeAPIError(w, http.StatusBadRequest, "password_mismatch", "Password confirmation does not match.")
		return
	}
	cookie, _ := r.Cookie(auth.SessionCookieName)
	if cookie == nil {
		writeAPIError(w, http.StatusUnauthorized, "unauthenticated", "Authentication is required.")
		return
	}
	if err := a.auth.ChangePassword(r.Context(), user, request.CurrentPassword, request.NewPassword, cookie.Value); err != nil {
		a.authError(w, "change password", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) requireAdmin(w http.ResponseWriter, r *http.Request) (auth.User, bool) {
	user, ok := a.requireUser(w, r)
	if !ok {
		return auth.User{}, false
	}
	if user.Role != auth.RoleAdmin {
		writeAPIError(w, http.StatusForbidden, "forbidden", "Administrator access is required.")
		return auth.User{}, false
	}
	return user, true
}
func adminTarget(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeAPIError(w, http.StatusBadRequest, "invalid_user_id", "Invalid user ID.")
		return 0, false
	}
	return id, true
}
func (a *API) adminUsers(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.requireAdmin(w, r); !ok {
		return
	}
	users, err := a.auth.ListUsers(r.Context(), r.URL.Query().Get("status"))
	if err != nil {
		a.authError(w, "list users", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"users": users})
}
func (a *API) adminApprove(w http.ResponseWriter, r *http.Request) {
	current, ok := a.requireAdmin(w, r)
	if !ok {
		return
	}
	id, ok := adminTarget(w, r)
	if !ok {
		return
	}
	if err := a.auth.Approve(r.Context(), current.ID, id); err != nil {
		a.authError(w, "approve user", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func (a *API) adminReject(w http.ResponseWriter, r *http.Request) {
	current, ok := a.requireAdmin(w, r)
	if !ok {
		return
	}
	id, ok := adminTarget(w, r)
	if !ok {
		return
	}
	var request rejectRequest
	if decodeJSON(w, r, &request) != nil {
		return
	}
	if err := a.auth.Reject(r.Context(), current.ID, id, request.Reason); err != nil {
		a.authError(w, "reject user", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func (a *API) adminResetPassword(w http.ResponseWriter, r *http.Request) {
	if !a.allowSensitive(w, r, "reset-password", 5) {
		return
	}
	if _, ok := a.requireAdmin(w, r); !ok {
		return
	}
	id, ok := adminTarget(w, r)
	if !ok {
		return
	}
	var request passwordConfirmation
	if decodeJSON(w, r, &request) != nil {
		return
	}
	if request.Password != request.ConfirmPassword {
		writeAPIError(w, http.StatusBadRequest, "password_mismatch", "Password confirmation does not match.")
		return
	}
	if err := a.auth.ResetPassword(r.Context(), id, request.Password); err != nil {
		a.authError(w, "reset password", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func (a *API) adminSetRole(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.requireAdmin(w, r); !ok {
		return
	}
	id, ok := adminTarget(w, r)
	if !ok {
		return
	}
	var request roleRequest
	if decodeJSON(w, r, &request) != nil {
		return
	}
	if err := a.auth.SetRole(r.Context(), id, request.Role); err != nil {
		a.authError(w, "set user role", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func (a *API) adminStatus(w http.ResponseWriter, r *http.Request, status string) {
	current, ok := a.requireAdmin(w, r)
	if !ok {
		return
	}
	id, ok := adminTarget(w, r)
	if !ok {
		return
	}
	if err := a.auth.SetStatus(r.Context(), current.ID, id, status); err != nil {
		a.authError(w, "update user status", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func (a *API) adminDisable(w http.ResponseWriter, r *http.Request) {
	a.adminStatus(w, r, auth.StatusDisabled)
}
func (a *API) adminEnable(w http.ResponseWriter, r *http.Request) {
	a.adminStatus(w, r, auth.StatusActive)
}
func (a *API) adminDelete(w http.ResponseWriter, r *http.Request) {
	a.adminStatus(w, r, auth.StatusDeleted)
}
func (a *API) adminRestore(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.requireAdmin(w, r); !ok {
		return
	}
	id, ok := adminTarget(w, r)
	if !ok {
		return
	}
	if err := a.auth.Restore(r.Context(), id); err != nil {
		a.authError(w, "restore user", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func (a *API) adminRevokeSessions(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.requireAdmin(w, r); !ok {
		return
	}
	id, ok := adminTarget(w, r)
	if !ok {
		return
	}
	if err := a.auth.RevokeUserSessions(r.Context(), id); err != nil {
		a.authError(w, "revoke sessions", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func (a *API) authError(w http.ResponseWriter, operation string, err error) {
	switch {
	case errors.Is(err, auth.ErrAlreadyInitialized):
		writeAPIError(w, http.StatusConflict, "already_initialized", err.Error())
	case errors.Is(err, auth.ErrSetupRequired):
		writeAPIError(w, http.StatusConflict, "setup_required", err.Error())
	case errors.Is(err, auth.ErrInvalidInput):
		writeAPIError(w, http.StatusBadRequest, "invalid_request", err.Error())
	case errors.Is(err, auth.ErrInvalidCredentials):
		writeAPIError(w, http.StatusUnauthorized, "invalid_credentials", err.Error())
	case errors.Is(err, auth.ErrApprovalPending):
		writeAPIError(w, http.StatusForbidden, "approval_pending", err.Error())
	case errors.Is(err, auth.ErrAccountDisabled):
		writeAPIError(w, http.StatusForbidden, "account_disabled", err.Error())
	case errors.Is(err, auth.ErrApplicationRejected):
		writeAPIError(w, http.StatusForbidden, "application_rejected", err.Error())
	case errors.Is(err, auth.ErrAccountDeleted):
		writeAPIError(w, http.StatusForbidden, "account_deleted", err.Error())
	case errors.Is(err, auth.ErrPlayerOffline):
		writeAPIError(w, http.StatusConflict, "player_not_online", err.Error())
	case errors.Is(err, auth.ErrPlayerNameAmbiguous):
		writeAPIError(w, http.StatusConflict, "player_name_ambiguous", err.Error())
	case errors.Is(err, auth.ErrPlayerIdentity):
		writeAPIError(w, http.StatusConflict, "player_identity_unavailable", err.Error())
	case errors.Is(err, auth.ErrUpstream):
		writeAPIError(w, http.StatusServiceUnavailable, "palworld_unavailable", err.Error())
	case errors.Is(err, auth.ErrDuplicateAccount):
		writeAPIError(w, http.StatusConflict, "duplicate_account", err.Error())
	case errors.Is(err, auth.ErrForbidden), errors.Is(err, auth.ErrUnsafeAdminAction):
		writeAPIError(w, http.StatusConflict, "forbidden", err.Error())
	case errors.Is(err, auth.ErrNotFound):
		writeAPIError(w, http.StatusNotFound, "not_found", err.Error())
	case errors.Is(err, auth.ErrInvalidTransition):
		writeAPIError(w, http.StatusConflict, "invalid_transition", err.Error())
	case errors.Is(err, auth.ErrInvalidPassword):
		writeAPIError(w, http.StatusBadRequest, "invalid_password", err.Error())
	default:
		if auth.IsExpected(err) {
			writeAPIError(w, http.StatusBadRequest, "invalid_request", err.Error())
			return
		}
		a.internal(w, operation, err)
	}
}
func (a *API) internal(w http.ResponseWriter, operation string, err error) {
	a.log.Error(operation+" failed", "error", err)
	writeAPIError(w, http.StatusInternalServerError, "internal_error", "Internal server error.")
}
