package serverstatus

import (
	"context"
	"sync"
	"time"
)

type CacheMeta struct {
	Cached    bool
	Stale     bool
	UpdatedAt time.Time
	HasValue  bool
}
type Cache[T any] struct {
	mu      sync.Mutex
	value   T
	updated time.Time
	ttl     time.Duration
	now     func() time.Time
	has     bool
}

func NewCache[T any](ttl time.Duration) *Cache[T] { return &Cache[T]{ttl: ttl, now: time.Now} }

func (c *Cache[T]) Get(ctx context.Context, load func(context.Context) (T, error)) (T, CacheMeta, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := c.now()
	if c.has && now.Sub(c.updated) < c.ttl {
		return c.value, CacheMeta{Cached: true, UpdatedAt: c.updated, HasValue: true}, nil
	}
	value, err := load(ctx)
	if err == nil {
		c.value, c.updated, c.has = value, now, true
		return value, CacheMeta{UpdatedAt: now, HasValue: true}, nil
	}
	if c.has {
		return c.value, CacheMeta{Cached: true, Stale: true, UpdatedAt: c.updated, HasValue: true}, err
	}
	var zero T
	return zero, CacheMeta{}, err
}
