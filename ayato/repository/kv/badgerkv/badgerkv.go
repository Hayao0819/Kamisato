// Package badgerkv implements kv.Store on top of an embedded BadgerDB. Keys are
// namespaced by joining ns and key with a NUL byte (ns + "\x00" + key), so a
// namespace's entries form a contiguous prefix range that List scans.
package badgerkv

import (
	"errors"
	"time"

	"github.com/Hayao0819/Kamisato/ayato/repository/kv"
	"github.com/Hayao0819/Kamisato/ayato/repository/kv/badgerkv/logger"
	"github.com/Hayao0819/Kamisato/internal/utils"
	"github.com/dgraph-io/badger/v3"
)

// sep separates the namespace from the key. NUL cannot appear in the namespaces
// or keys callers use (package names, hex digests, ids), so it is a safe and
// unambiguous boundary for the prefix scan List relies on.
const sep = "\x00"

// Store is a kv.Store backed by BadgerDB.
type Store struct {
	db *badger.DB
}

// compile-time interface check.
var _ kv.Store = (*Store)(nil)

// New opens (or creates) a BadgerDB at dir and returns a kv.Store over it.
func New(dir string) (*Store, error) {
	opt := badger.DefaultOptions(dir)
	opt.Logger = logger.Default()

	db, err := badger.Open(opt)
	if err != nil {
		return nil, utils.WrapErr(err, "badgerkv: open badger")
	}
	return &Store{db: db}, nil
}

func composite(ns, key string) []byte {
	return []byte(ns + sep + key)
}

func nsPrefix(ns string) []byte {
	return []byte(ns + sep)
}

// Get returns the value under (ns, key), or kv.ErrNotFound when it is absent or
// expired.
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
		return nil, utils.WrapErr(err, "badgerkv: get")
	}
	return out, nil
}

// Set stores value under (ns, key). A positive ttl makes the entry expire after
// that duration; ttl == 0 stores it without expiry.
func (s *Store) Set(ns, key string, value []byte, ttl time.Duration) error {
	return s.db.Update(func(txn *badger.Txn) error {
		e := badger.NewEntry(composite(ns, key), value)
		if ttl > 0 {
			e = e.WithTTL(ttl)
		}
		return txn.SetEntry(e)
	})
}

// Delete removes (ns, key). Removing a missing key is not an error.
func (s *Store) Delete(ns, key string) error {
	return s.db.Update(func(txn *badger.Txn) error {
		return txn.Delete(composite(ns, key))
	})
}

// List returns every live entry within ns by scanning the namespace prefix and
// stripping it from each returned key.
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
		return nil, utils.WrapErr(err, "badgerkv: list")
	}
	return out, nil
}

// Close releases the underlying BadgerDB.
func (s *Store) Close() error { return s.db.Close() }
