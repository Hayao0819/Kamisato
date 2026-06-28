// Package kv is a generic, namespaced string->bytes key-value abstraction shared
// by ayato's persistence layers. It deliberately knows nothing about package
// metadata, auth, or any other domain: callers partition their data by namespace
// and ride a single Store implementation (BadgerDB, Cloudflare Workers KV, or
// SQL). Each backend maps absence to ErrNotFound and honours an optional TTL.
package kv

import (
	"errors"
	"time"
)

//go:generate mockgen -source=kv.go -destination=../../test/mocks/kv.go -package=mocks -mock_names Store=MockKVStore

// ErrNotFound is returned by Get when a key is absent (or has expired). Backends
// never report a miss as ("", nil); they always surface it as ErrNotFound so
// callers can branch uniformly with errors.Is.
var ErrNotFound = errors.New("kv: not found")

// Key is the bare key within the namespace (the namespace prefix is stripped).
type Entry struct {
	Key   string
	Value []byte
}

// Store implementations must be safe for concurrent use. A namespace (ns)
// partitions the keyspace so unrelated domains (package metadata, auth, ...)
// never collide.
type Store interface {
	// Get returns ErrNotFound when the key is absent or expired; it never
	// returns ("", nil) for a miss.
	Get(ns, key string) ([]byte, error)
	// A ttl of 0 means no expiry; a positive ttl makes the entry expire after
	// that duration.
	Set(ns, key string, value []byte, ttl time.Duration) error
	// Deleting a missing key is not an error.
	Delete(ns, key string) error
	// List returns every live entry within ns (a prefix scan over the namespace).
	List(ns string) ([]Entry, error)
	Close() error
}
