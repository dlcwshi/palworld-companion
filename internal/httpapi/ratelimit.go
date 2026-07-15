package httpapi

import (
	"net"
	"net/http"
	"sync"
	"time"
)

type rateEntry struct {
	count int
	reset time.Time
}
type rateLimiter struct {
	mu      sync.Mutex
	entries map[string]rateEntry
	now     func() time.Time
}

func newRateLimiter() *rateLimiter {
	return &rateLimiter{entries: make(map[string]rateEntry), now: time.Now}
}
func (l *rateLimiter) allow(key string, limit int, window time.Duration) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	entry, ok := l.entries[key]
	if !ok || !entry.reset.After(now) {
		l.entries[key] = rateEntry{count: 1, reset: now.Add(window)}
		return true
	}
	if entry.count >= limit {
		return false
	}
	entry.count++
	l.entries[key] = entry
	if len(l.entries) > 4096 {
		for k, v := range l.entries {
			if !v.reset.After(now) {
				delete(l.entries, k)
			}
		}
	}
	return true
}
func requestIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return r.RemoteAddr
}
func (a *API) allowSensitive(w http.ResponseWriter, r *http.Request, operation string, limit int) bool {
	if a.limiter.allow(operation+":"+requestIP(r), limit, time.Minute) {
		return true
	}
	w.Header().Set("Retry-After", "60")
	writeAPIError(w, http.StatusTooManyRequests, "rate_limited", "Too many requests. Please try again later.")
	return false
}
