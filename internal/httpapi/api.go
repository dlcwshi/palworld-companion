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
	status *serverstatus.Service
	tasks  *tasks.Service
	build  BuildInfo
	log    *slog.Logger
	assets fs.FS
	static http.Handler
}

func New(status *serverstatus.Service, taskService *tasks.Service, build BuildInfo, logger *slog.Logger, assets fs.FS) http.Handler {
	api := &API{status: status, tasks: taskService, build: build, log: logger, assets: assets}
	if assets != nil {
		api.static = http.FileServer(http.FS(assets))
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/health", api.health)
	mux.HandleFunc("GET /api/v1/system/version", api.version)
	mux.HandleFunc("GET /api/v1/system/capabilities", api.capabilities)
	mux.HandleFunc("GET /api/v1/server/summary", api.summary)
	mux.HandleFunc("GET /api/v1/server/players", api.players)
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
	writeJSON(w, http.StatusOK, map[string]any{"palworld": map[string]bool{"info": true, "metrics": true, "players": true}, "features": map[string]bool{"crafting": false, "breeding": false, "map": false, "tasks": true}})
}
func (a *API) summary(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, a.status.Summary(r.Context()))
}
func (a *API) players(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, a.status.Players(r.Context()))
}

type taskCreateRequest struct {
	Title string `json:"title"`
	Notes string `json:"notes"`
}
type taskUpdateRequest struct {
	Title     *string `json:"title"`
	Notes     *string `json:"notes"`
	Status    *string `json:"status"`
	SortOrder *int    `json:"sortOrder"`
}

func (a *API) listTasks(w http.ResponseWriter, r *http.Request) {
	limit := 0
	if raw := r.URL.Query().Get("limit"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil {
			writeError(w, http.StatusBadRequest, "limit must be an integer")
			return
		}
		limit = value
	}
	result, err := a.tasks.List(r.Context(), tasks.ListOptions{Status: r.URL.Query().Get("status"), Limit: limit})
	if err != nil {
		a.taskError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}
func (a *API) createTask(w http.ResponseWriter, r *http.Request) {
	var request taskCreateRequest
	if err := decodeJSON(w, r, &request); err != nil {
		return
	}
	task, err := a.tasks.Create(r.Context(), tasks.CreateInput{Title: request.Title, Notes: request.Notes})
	if err != nil {
		a.taskError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, task)
}
func (a *API) getTask(w http.ResponseWriter, r *http.Request) {
	id, ok := taskID(w, r)
	if !ok {
		return
	}
	task, err := a.tasks.Get(r.Context(), id)
	if err != nil {
		a.taskError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, task)
}
func (a *API) updateTask(w http.ResponseWriter, r *http.Request) {
	id, ok := taskID(w, r)
	if !ok {
		return
	}
	var request taskUpdateRequest
	if err := decodeJSON(w, r, &request); err != nil {
		return
	}
	task, err := a.tasks.Update(r.Context(), id, tasks.UpdateInput{Title: request.Title, Notes: request.Notes, Status: request.Status, SortOrder: request.SortOrder})
	if err != nil {
		a.taskError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, task)
}
func (a *API) deleteTask(w http.ResponseWriter, r *http.Request) {
	id, ok := taskID(w, r)
	if !ok {
		return
	}
	if err := a.tasks.Delete(r.Context(), id); err != nil {
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
		a.static.ServeHTTP(w, r)
		return
	}
	index, err := fs.ReadFile(a.assets, "index.html")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(index)
}
func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
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
