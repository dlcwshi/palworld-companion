package httpapi

import (
	"encoding/json"
	"io/fs"
	"log/slog"
	"net/http"
	"path"
	"strings"

	"github.com/dlcwshi/palworld-companion/internal/serverstatus"
)

type BuildInfo struct {
	Name      string `json:"name"`
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	BuildTime string `json:"buildTime"`
}
type API struct {
	status *serverstatus.Service
	build  BuildInfo
	log    *slog.Logger
	assets fs.FS
	static http.Handler
}

func New(status *serverstatus.Service, build BuildInfo, logger *slog.Logger, assets fs.FS) http.Handler {
	api := &API{status: status, build: build, log: logger, assets: assets}
	if assets != nil {
		api.static = http.FileServer(http.FS(assets))
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/health", api.health)
	mux.HandleFunc("GET /api/v1/system/version", api.version)
	mux.HandleFunc("GET /api/v1/system/capabilities", api.capabilities)
	mux.HandleFunc("GET /api/v1/server/summary", api.summary)
	mux.HandleFunc("GET /api/v1/server/players", api.players)
	mux.HandleFunc("/", api.frontend)
	return requestLogger(logger, secureHeaders(mux))
}

func (a *API) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "version": a.build.Version})
}
func (a *API) version(w http.ResponseWriter, _ *http.Request) { writeJSON(w, http.StatusOK, a.build) }
func (a *API) capabilities(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"palworld": map[string]bool{"info": true, "metrics": true, "players": true},
		"features": map[string]bool{"crafting": false, "breeding": false, "map": false, "tasks": false},
	})
}
func (a *API) summary(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, a.status.Summary(r.Context()))
}
func (a *API) players(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, a.status.Players(r.Context()))
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
