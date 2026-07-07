//go:build !js

// BadgerDB cannot build on js/wasm: it needs unix syscalls (mmap).

// Package badgerkv implements kv.Store on top of an embedded BadgerDB. Keys are
// namespaced by joining ns and key with a NUL byte (ns + "\x00" + key), so a
// namespace's entries form a contiguous prefix range that List scans.
package badgerkv

import (
	"errors"
	"log/slog"
	"math/bits"
	"sync"
	"time"

	"github.com/Hayao0819/Kamisato/ayato/repository/kv"
	"github.com/Hayao0819/Kamisato/ayato/repository/kv/badgerkv/logger"
	"github.com/Hayao0819/Kamisato/internal/errwrap"
	"github.com/dgraph-io/badger/v4"
)

// NUL cannot appear in the namespaces or keys callers use (package names, hex
// digests, ids), so it is a safe, unambiguous boundary for the prefix scan List
// relies on.
const sep = "\x00"

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
	opt.Logger = logger.Default()
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
		return nil, errwrap.WrapErr(err, "badgerkv: open badger")
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

func composite(ns, key string) []byte {
	return []byte(ns + sep + key)
}

func nsPrefix(ns string) []byte {
	return []byte(ns + sep)
}

func (s *Store) Get(ns, key string) ([]byte, error) {
	var out []byte
	err := s.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(composite(ns, key))
		if errors.Is(err, badger.ErrKeyNotFound) {
			return kv.ErrNotFound
		}
		if err != nil {
			return err
		}
		out, err = item.ValueCopy(nil)
		return err
	})
	if err != nil {
		if errors.Is(err, kv.ErrNotFound) {
			return nil, kv.ErrNotFound
		}
		return nil, errwrap.WrapErr(err, "badgerkv: get")
	}
	return out, nil
}

func (s *Store) Set(ns, key string, value []byte, ttl time.Duration) error {
	return s.db.Update(func(txn *badger.Txn) error {
		e := badger.NewEntry(composite(ns, key), value)
		if ttl > 0 {
			e = e.WithTTL(ttl)
		}
		return txn.SetEntry(e)
	})
}

func (s *Store) Delete(ns, key string) error {
	return s.db.Update(func(txn *badger.Txn) error {
		return txn.Delete(composite(ns, key))
	})
}

// Add sets key only when it is absent. Badger's serializable transaction records
// the read of a missing key, so two racing Adds of the same key cannot both commit
// with created=true: one wins and the other's Commit conflicts (surfaced as an
// error), which fails closed rather than allowing a double-insert.
func (s *Store) Add(ns, key string, value []byte, ttl time.Duration) (bool, error) {
	created := false
	err := s.db.Update(func(txn *badger.Txn) error {
		_, gerr := txn.Get(composite(ns, key))
		if gerr == nil {
			created = false
			return nil
		}
		if !errors.Is(gerr, badger.ErrKeyNotFound) {
			return gerr
		}
		e := badger.NewEntry(composite(ns, key), value)
		if ttl > 0 {
			e = e.WithTTL(ttl)
		}
		created = true
		return txn.SetEntry(e)
	})
	if err != nil {
		return false, errwrap.WrapErr(err, "badgerkv: add")
	}
	return created, nil
}

func (s *Store) List(ns string) ([]kv.Entry, error) {
	prefix := nsPrefix(ns)
	var out []kv.Entry
	err := s.db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()
		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()
			val, verr := item.ValueCopy(nil)
			if verr != nil {
				return verr
			}
			key := string(item.KeyCopy(nil)[len(prefix):])
			out = append(out, kv.Entry{Key: key, Value: val})
		}
		return nil
	})
	if err != nil {
		return nil, errwrap.WrapErr(err, "badgerkv: list")
	}
	return out, nil
}

func (s *Store) Close() error {
	s.stopOnce.Do(func() { close(s.stop) })
	s.wg.Wait()
	return s.db.Close()
}
