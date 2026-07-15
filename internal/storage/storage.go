// Package storage reserves the persistence boundary for future Companion
// features. v0.1.0 intentionally ships without a database implementation or
// business tables.
package storage

import "context"

// Store is the minimal lifecycle contract a future SQLite implementation must
// satisfy.
type Store interface {
	Ping(context.Context) error
	Close() error
}