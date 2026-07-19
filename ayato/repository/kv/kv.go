// Package kv is a generic, namespaced string->bytes key-value abstraction shared
// by ayato's persistence layers. Each backend (BadgerDB, Workers KV, SQL) maps
// absence to ErrNotFound and honours an optional TTL.
package kv

import (
	"fmt"
	"log/slog"
	"strings"
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

// BulkStore is an optional capability: batched writes/deletes a backend can send in
// one request, so a large migration mutation stays within a remote store's
// per-account request budget. Backends without it are driven per-key.
type BulkStore interface {
	BulkSet(ns string, entries []Entry, ttl time.Duration) error
	BulkDelete(ns string, keys []string) error
}

// KeyAuditor is an optional capability for finding entries a backend holds that the
// application did not create — e.g. data injected through a provider's console. Each
// backend judges by its own key format, so no namespace allowlist is needed.
type KeyAuditor interface {
	ForeignKeys() ([]string, error)
	DeleteRawKeys(keys []string) error
}

// Adder is an optional Store capability: atomically set key only when it is
// absent, reporting whether this call created it. It is the primitive a one-time /
// replay guard needs — two racing callers cannot both observe "created". Backends
// that cannot insert atomically (an eventually-consistent store) simply do not
// implement it; security-sensitive consumers must fail closed.
type Adder interface {
	Add(ns, key string, value []byte, ttl time.Duration) (created bool, err error)
}

// PrintfLogger adapts slog to the printf-style logger interfaces used by KV
// backend clients.
type PrintfLogger struct {
	logger *slog.Logger
}

func NewPrintfLogger(logger *slog.Logger) *PrintfLogger {
	if logger == nil {
		logger = slog.Default()
	}
	return &PrintfLogger{logger: logger}
}

func printfMessage(format string, args ...interface{}) string {
	return strings.TrimSuffix(fmt.Sprintf(format, args...), "\n")
}

func (l *PrintfLogger) Printf(format string, args ...interface{}) {
	l.logger.Info(printfMessage(format, args...))
}

func (l *PrintfLogger) Errorf(format string, args ...interface{}) {
	l.logger.Error(printfMessage(format, args...))
}

func (l *PrintfLogger) Warningf(format string, args ...interface{}) {
	l.logger.Warn(printfMessage(format, args...))
}

func (l *PrintfLogger) Infof(format string, args ...interface{}) {
	l.logger.Info(printfMessage(format, args...))
}

func (l *PrintfLogger) Debugf(format string, args ...interface{}) {
	l.logger.Debug(printfMessage(format, args...))
}
