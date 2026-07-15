package serverstatus

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestCacheHitExpiryAndStaleFallback(t *testing.T) {
	now := time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)
	cache := NewCache[string](5 * time.Second)
	cache.now = func() time.Time { return now }
	calls := 0
	load := func(context.Context) (string, error) { calls++; return "fresh", nil }
	value, meta, err := cache.Get(context.Background(), load)
	if err != nil || value != "fresh" || meta.Cached || calls != 1 {
		t.Fatalf("first get: %q %+v %v calls=%d", value, meta, err, calls)
	}
	value, meta, err = cache.Get(context.Background(), load)
	if err != nil || !meta.Cached || meta.Stale || calls != 1 {
		t.Fatalf("cache hit: %q %+v %v calls=%d", value, meta, err, calls)
	}
	now = now.Add(6 * time.Second)
	upstream := errors.New("offline")
	value, meta, err = cache.Get(context.Background(), func(context.Context) (string, error) { calls++; return "", upstream })
	if !errors.Is(err, upstream) || value != "fresh" || !meta.Cached || !meta.Stale || !meta.HasValue {
		t.Fatalf("stale fallback: %q %+v %v", value, meta, err)
	}
}
