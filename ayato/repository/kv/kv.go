// Package kv is a generic, namespaced string->bytes key-value abstraction shared
// by ayato's persistence layers. Each backend (BadgerDB, Workers KV, SQL) maps
// absence to ErrNotFound and honours an optional TTL.
package kv

import (
	"time"

	"github.com/Hayao0819/Kamisato/internal/errors"
)

//go:generate mockgen -source=kv.go -destination=../../test/mocks/kv.go -package=mocks -mock_names Store=MockKVStore

// ErrNotFound is returned by Get when a key is absent or expired. Backends never
// report a miss as ("", nil), so callers can branch uniformly with errors.Is.
var ErrNotFound = errors.New("kv: not found")

// Entry's Key has the namespace prefix stripped.
type Entry struct {
	Key   string
	Value []byte
}

// Store implementations must be safe for concurrent use. A namespace (ns)
// partitions the keyspace so unrelated domains never collide.
type Store interface {
	// Get returns ErrNotFound when the key is absent or expired.
	Get(ns, key string) ([]byte, error)
	// A ttl of 0 means no expiry; a positive ttl expires the entry after that long.
	Set(ns, key string, value []byte, ttl time.Duration) error
	// Deleting a missing key is not an error.
	Delete(ns, key string) error
	// List returns every live (non-expired) entry within ns.
	List(ns string) ([]Entry, error)
	Close() error
}

// Adder is an optional Store capability: atomically set key only when it is
// absent, reporting whether this call created it. It is the primitive a one-time /
// replay guard needs — two racing callers cannot both observe "created". Backends
// that cannot insert atomically (an eventually-consistent store) simply do not
// implement it, and callers fall back to a Get-then-Set check.
type Adder interface {
	Add(ns, key string, value []byte, ttl time.Duration) (created bool, err error)
}
