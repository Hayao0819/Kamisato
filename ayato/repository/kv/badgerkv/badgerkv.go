//go:build !js

// BadgerDB cannot build on js/wasm: it needs unix syscalls (mmap).

// Package badgerkv implements kv.Store on top of an embedded BadgerDB. Keys are
// namespaced by joining ns and key with a NUL byte (ns + "\x00" + key), so a
// namespace's entries form a contiguous prefix range that List scans.
package badgerkv

import (
	"log/slog"
	"math/bits"
	"sync"
	"time"

	"github.com/Hayao0819/Kamisato/internal/errors"

	"github.com/dgraph-io/badger/v4"

	"github.com/Hayao0819/Kamisato/ayato/repository/kv"
	"github.com/Hayao0819/Kamisato/ayato/repository/kv/slogadapter"
)

// BadgerDB only frees space from deleted/overwritten keys when RunValueLogGC is
// called, so a background ticker reclaims stale value-log segments to keep the
// on-disk store from growing without bound on a long-running deployment.
const (
	gcInterval     = 5 * time.Minute
	gcDiscardRatio = 0.5
)

type Store struct {
	db *badger.DB

	stop     chan struct{}
	stopOnce sync.Once
	wg       sync.WaitGroup
}

var _ kv.Store = (*Store)(nil)

func New(dir string) (*Store, error) {
	opt := badger.DefaultOptions(dir)
	opt.Logger = slogadapter.Default()
	// BadgerDB memory-maps each value-log file at twice its configured size
	// (~2 GiB by default). That mapping does not fit — let alone repeatedly — in a
	// 32-bit process's limited address space, so on 32-bit builds cap the value-log
	// file size to keep the mmap small. amd64/arm64 keep the default. bits.UintSize
	// is a constant, so the other branch is compiled out.
	if bits.UintSize == 32 {
		opt = opt.WithValueLogFileSize(64 << 20)
	}

	db, err := badger.Open(opt)
	if err != nil {
		return nil, errors.WrapErr(err, "badgerkv: open badger")
	}
	s := &Store{db: db, stop: make(chan struct{})}
	s.wg.Add(1)
	go s.gcLoop()
	return s, nil
}

// gcLoop reclaims value-log space on a ticker until Close signals stop.
func (s *Store) gcLoop() {
	defer s.wg.Done()
	ticker := time.NewTicker(gcInterval)
	defer ticker.Stop()
	for {
		select {
		case <-s.stop:
			return
		case <-ticker.C:
			s.collectGarbage()
		}
	}
}

// collectGarbage rewrites reclaimable value-log files one at a time until there
// is nothing left to rewrite or Close is requested.
func (s *Store) collectGarbage() {
	for {
		select {
		case <-s.stop:
			return
		default:
		}
		err := s.db.RunValueLogGC(gcDiscardRatio)
		switch {
		case err == nil:
			// Rewrote a file; another may be reclaimable too.
			continue
		case errors.Is(err, badger.ErrNoRewrite):
			return
		default:
			// ErrRejected (concurrent GC) or a shutdown error: retry next tick.
			slog.Debug("badgerkv: value-log GC", "error", err)
			return
		}
	}
}

func (s *Store) Close() error {
	s.stopOnce.Do(func() { close(s.stop) })
	s.wg.Wait()
	return s.db.Close()
}
