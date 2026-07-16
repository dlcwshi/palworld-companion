package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"path"
	"strconv"
	"strings"

	"github.com/dlcwshi/palworld-companion/internal/auth"
	"github.com/dlcwshi/palworld-companion/internal/serverstatus"
	"github.com/dlcwshi/palworld-companion/internal/tasks"
)

type BuildInfo struct {
	Name      string `json:"name"`
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	BuildTime string `json:"buildTime"`
}
type API struct {
	status  *serverstatus.Service
	tasks   *tasks.Service
	auth    *auth.Service
	build   BuildInfo
	log     *slog.Logger
	assets  fs.FS
	static  http.Handler
	limiter *rateLimiter
}

func New(status *serverstatus.Service, taskService *tasks.Service, authService *auth.Service, build BuildInfo, logger *slog.Logger, assets fs.FS) http.Handler {
	api := &API{status: status, tasks: taskService, auth: authService, build: build, log: logger, assets: assets, limiter: newRateLimiter()}
	if assets != nil {
		api.static = http.FileServer(http.FS(assets))
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/health", api.health)
	mux.HandleFunc("GET /api/v1/system/version", api.version)
	mux.HandleFunc("GET /api/v1/system/capabilities", api.capabilities)
	mux.HandleFunc("GET /api/v1/server/summary", api.summary)
	mux.HandleFunc("GET /api/v1/server/players", api.players)
	mux.HandleFunc("GET /api/v1/setup/status", api.setupStatus)
	mux.HandleFunc("POST /api/v1/setup/admin", api.setupAdmin)
	mux.HandleFunc("GET /api/v1/auth/steam", api.steamDisabled)
	mux.HandleFunc("GET /api/v1/auth/steam/callback", api.steamDisabled)
	mux.HandleFunc("POST /api/v1/auth/register", api.register)
	mux.HandleFunc("POST /api/v1/auth/login", api.login)
	mux.HandleFunc("POST /api/v1/auth/change-password", api.changePassword)
	mux.HandleFunc("GET /api/v1/auth/me", api.me)
	mux.HandleFunc("POST /api/v1/auth/logout", api.logout)
	mux.HandleFunc("GET /api/v1/admin/users", api.adminUsers)
	mux.HandleFunc("POST /api/v1/admin/users/{id}/approve", api.adminApprove)
	mux.HandleFunc("POST /api/v1/admin/users/{id}/reject", api.adminReject)
	mux.HandleFunc("POST /api/v1/admin/users/{id}/reset-password", api.adminResetPassword)
	mux.HandleFunc("POST /api/v1/admin/users/{id}/role", api.adminSetRole)
	mux.HandleFunc("POST /api/v1/admin/users/{id}/disable", api.adminDisable)
	mux.HandleFunc("POST /api/v1/admin/users/{id}/enable", api.adminEnable)
	mux.HandleFunc("DELETE /api/v1/admin/users/{id}", api.adminDelete)
	mux.HandleFunc("POST /api/v1/admin/users/{id}/restore", api.adminRestore)
	mux.HandleFunc("POST /api/v1/admin/users/{id}/revoke-sessions", api.adminRevokeSessions)
	mux.HandleFunc("GET /api/v1/tasks", api.listTasks)
	mux.HandleFunc("POST /api/v1/tasks", api.createTask)
	mux.HandleFunc("GET /api/v1/tasks/{id}", api.getTask)
	mux.HandleFunc("PATCH /api/v1/tasks/{id}", api.updateTask)
	mux.HandleFunc("DELETE /api/v1/tasks/{id}", api.deleteTask)
	mux.HandleFunc("/", api.frontend)
	return requestLogger(logger, secureHeaders(mux))
}

func (a *API) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "version": a.build.Version})
}
func (a *API) version(w http.ResponseWriter, _ *http.Request) { writeJSON(w, http.StatusOK, a.build) }
func (a *API) capabilities(w http.ResponseWriter, _ *http.Request) {
	enabled := a.auth != nil
	writeJSON(w, http.StatusOK, map[string]any{"palworld": map[string]bool{"info": true, "metrics": true, "players": true}, "features": map[string]bool{"crafting": false, "breeding": false, "map": false, "tasks": enabled, "steamAuth": false, "localAuth": enabled, "initialSetup": enabled, "playerRegistration": enabled, "userAccounts": enabled, "taskOwnership": enabled, "adminUsers": enabled}})
}
func (a *API) summary(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, a.status.Summary(r.Context()))
}
func (a *API) players(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, a.status.Players(r.Context()))
}

type taskCreateRequest struct {
	Title      string `json:"title"`
	Notes      string `json:"notes"`
	Visibility string `json:"visibility"`
}
type taskUpdateRequest struct {
	Title     *string `json:"title"`
	Notes     *string `json:"notes"`
	Status    *string `json:"status"`
	SortOrder *int    `json:"sortOrder"`
}

func (a *API) listTasks(w http.ResponseWriter, r *http.Request) {
	user, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	limit := 0
	if raw := r.URL.Query().Get("limit"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil {
			writeError(w, http.StatusBadRequest, "limit must be an integer")
			return
		}
		limit = value
	}
	result, err := a.tasks.ListFor(r.Context(), tasks.Actor{ID: user.ID, Role: user.Role}, tasks.ListOptions{Status: r.URL.Query().Get("status"), Limit: limit, Scope: r.URL.Query().Get("scope")})
	if err != nil {
		a.taskError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}
func (a *API) createTask(w http.ResponseWriter, r *http.Request) {
	user, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	var request taskCreateRequest
	if err := decodeJSON(w, r, &request); err != nil {
		return
	}
	task, err := a.tasks.CreateFor(r.Context(), tasks.Actor{ID: user.ID, Role: user.Role}, tasks.CreateInput{Title: request.Title, Notes: request.Notes, Visibility: request.Visibility})
	if err != nil {
		a.taskError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, task)
}
func (a *API) getTask(w http.ResponseWriter, r *http.Request) {
	user, authenticated := a.requireUser(w, r)
	if !authenticated {
		return
	}
	id, ok := taskID(w, r)
	if !ok {
		return
	}
	task, err := a.tasks.GetFor(r.Context(), tasks.Actor{ID: user.ID, Role: user.Role}, id)
	if err != nil {
		a.taskError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, task)
}
func (a *API) updateTask(w http.ResponseWriter, r *http.Request) {
	user, authenticated := a.requireUser(w, r)
	if !authenticated {
		return
	}
	id, ok := taskID(w, r)
	if !ok {
		return
	}
	var request taskUpdateRequest
	if err := decodeJSON(w, r, &request); err != nil {
		return
	}
	task, err := a.tasks.UpdateFor(r.Context(), tasks.Actor{ID: user.ID, Role: user.Role}, id, tasks.UpdateInput{Title: request.Title, Notes: request.Notes, Status: request.Status, SortOrder: request.SortOrder})
	if err != nil {
		a.taskError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, task)
}
func (a *API) deleteTask(w http.ResponseWriter, r *http.Request) {
	user, authenticated := a.requireUser(w, r)
	if !authenticated {
		return
	}
	id, ok := taskID(w, r)
	if !ok {
		return
	}
	if err := a.tasks.DeleteFor(r.Context(), tasks.Actor{ID: user.ID, Role: user.Role}, id); err != nil {
		a.taskError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func (a *API) taskError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, tasks.ErrInvalidInput):
		writeError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, tasks.ErrNotFound):
		writeError(w, http.StatusNotFound, "task not found")
	default:
		a.log.Error("task request failed", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
	}
}
func taskID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "invalid task id")
		return 0, false
	}
	return id, true
}
func decodeJSON(w http.ResponseWriter, r *http.Request, target any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON request")
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		writeError(w, http.StatusBadRequest, "request must contain one JSON object")
		return fmt.Errorf("extra JSON content")
	}
	return nil
}

func (a *API) frontend(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/api/") || a.static == nil {
		http.NotFound(w, r)
		return
	}
	clean := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
	if clean == "." {
		clean = "index.html"
	}
	if file, err := fs.Stat(a.assets, clean); err == nil && !file.IsDir() {
		setFrontendCacheHeaders(w, clean)
		a.static.ServeHTTP(w, r)
		return
	}
	index, err := fs.ReadFile(a.assets, "index.html")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	setFrontendCacheHeaders(w, "index.html")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(index)
}
func setFrontendCacheHeaders(w http.ResponseWriter, name string) {
	if strings.HasPrefix(name, "assets/") {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		return
	}
	w.Header().Set("Cache-Control", "no-cache")
}
func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
func writeAPIError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]string{"code": code, "message": message})
}
func secureHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; connect-src 'self'; manifest-src 'self'")
		next.ServeHTTP(w, r)
	})
}
func requestLogger(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logger.Info("http request", "method", r.Method, "path", r.URL.Path, "remote", r.RemoteAddr)
		next.ServeHTTP(w, r)
	})
}
