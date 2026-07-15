package httpapi

import (
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/dlcwshi/palworld-companion/internal/auth"
)

func (a *API) steamLogin(w http.ResponseWriter, r *http.Request) {
	if a.auth == nil || !a.auth.Enabled() {
		writeError(w, http.StatusServiceUnavailable, "Steam 登录尚未配置")
		return
	}
	location, state, err := a.auth.Begin(r.Context(), r.URL.Query().Get("returnTo"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	http.SetCookie(w, &http.Cookie{Name: auth.StateCookieName, Value: state, Path: "/api/v1/auth/steam/callback", MaxAge: 600, HttpOnly: true, Secure: true, SameSite: http.SameSiteLaxMode})
	http.Redirect(w, r, location, http.StatusFound)
}

func callbackCode(err error) string {
	switch {
	case errors.Is(err, auth.ErrPlayerOffline):
		return "player_offline"
	case errors.Is(err, auth.ErrUpstream):
		return "palworld_unavailable"
	case errors.Is(err, auth.ErrAccountDisabled):
		return "account_disabled"
	case errors.Is(err, auth.ErrAccountDeleted):
		return "account_deleted"
	case errors.Is(err, auth.ErrInvalidFlow):
		return "invalid_flow"
	default:
		return "steam_verification_failed"
	}
}

func (a *API) steamCallback(w http.ResponseWriter, r *http.Request) {
	cookie, _ := r.Cookie(auth.StateCookieName)
	cookieState := ""
	if cookie != nil {
		cookieState = cookie.Value
	}
	_, token, returnPath, err := a.auth.Callback(r.Context(), r.URL.Query(), cookieState)
	http.SetCookie(w, &http.Cookie{Name: auth.StateCookieName, Value: "", Path: "/api/v1/auth/steam/callback", MaxAge: -1, HttpOnly: true, Secure: true, SameSite: http.SameSiteLaxMode})
	if err != nil {
		a.log.Warn("Steam login failed", "reason", callbackCode(err))
		http.Redirect(w, r, "/login?error="+url.QueryEscape(callbackCode(err)), http.StatusSeeOther)
		return
	}
	ttl := a.auth.SessionTTL()
	http.SetCookie(w, &http.Cookie{Name: auth.SessionCookieName, Value: token, Path: "/", Expires: time.Now().Add(ttl), MaxAge: int(ttl.Seconds()), HttpOnly: true, Secure: true, SameSite: http.SameSiteLaxMode})
	http.Redirect(w, r, returnPath, http.StatusSeeOther)
}

func (a *API) currentUser(r *http.Request) (auth.User, error) {
	if a.auth == nil || !a.auth.Enabled() {
		return auth.User{}, auth.ErrAuthDisabled
	}
	cookie, err := r.Cookie(auth.SessionCookieName)
	if err != nil {
		return auth.User{}, auth.ErrUnauthenticated
	}
	return a.auth.Authenticate(r.Context(), cookie.Value)
}
func (a *API) requireUser(w http.ResponseWriter, r *http.Request) (auth.User, bool) {
	user, err := a.currentUser(r)
	if err == nil {
		return user, true
	}
	if errors.Is(err, auth.ErrAuthDisabled) {
		writeError(w, http.StatusServiceUnavailable, "Steam 登录尚未配置")
	} else {
		writeError(w, http.StatusUnauthorized, "authentication required")
	}
	return auth.User{}, false
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
	if cookie != nil {
		_ = a.auth.Logout(r.Context(), cookie.Value)
	}
	http.SetCookie(w, &http.Cookie{Name: auth.SessionCookieName, Value: "", Path: "/", MaxAge: -1, HttpOnly: true, Secure: true, SameSite: http.SameSiteLaxMode})
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) requireAdmin(w http.ResponseWriter, r *http.Request) (auth.User, bool) {
	user, ok := a.requireUser(w, r)
	if !ok {
		return auth.User{}, false
	}
	if user.Role != auth.RoleAdmin {
		writeError(w, http.StatusForbidden, "forbidden")
		return auth.User{}, false
	}
	return user, true
}
func adminTarget(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "invalid user id")
		return 0, false
	}
	return id, true
}
func (a *API) adminUsers(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.requireAdmin(w, r); !ok {
		return
	}
	users, err := a.auth.ListUsers(r.Context())
	if err != nil {
		a.internal(w, "list users", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"users": users})
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
		if errors.Is(err, auth.ErrUnsafeAdminAction) {
			writeError(w, http.StatusConflict, err.Error())
		} else {
			a.internal(w, "update user", err)
		}
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
	a.adminStatus(w, r, auth.StatusActive)
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
		a.internal(w, "revoke sessions", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func (a *API) internal(w http.ResponseWriter, operation string, err error) {
	a.log.Error(operation+" failed", "error", err)
	writeError(w, http.StatusInternalServerError, "internal server error")
}
