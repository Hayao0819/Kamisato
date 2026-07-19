//go:build !js

package badgerkv

import (
	"time"

	"github.com/dgraph-io/badger/v4"

	"github.com/Hayao0819/Kamisato/ayato/repository/kv"
	"github.com/Hayao0819/Kamisato/internal/errors"
)

// NUL cannot appear in the namespaces or keys callers use, so it is a safe,
// unambiguous boundary for prefix scans.
const sep = "\x00"

func composite(namespace, key string) []byte {
	return []byte(namespace + sep + key)
}

func nsPrefix(namespace string) []byte {
	return []byte(namespace + sep)
}

func (s *Store) Get(namespace, key string) ([]byte, error) {
	var value []byte
	err := s.db.View(func(transaction *badger.Txn) error {
		item, err := transaction.Get(composite(namespace, key))
		if errors.Is(err, badger.ErrKeyNotFound) {
			return kv.ErrNotFound
		}
		if err != nil {
			return err
		}
		value, err = item.ValueCopy(nil)
		return err
	})
	if err != nil {
		if errors.Is(err, kv.ErrNotFound) {
			return nil, kv.ErrNotFound
		}
		return nil, errors.WrapErr(err, "badgerkv: get")
	}
	return value, nil
}

func (s *Store) Set(namespace, key string, value []byte, ttl time.Duration) error {
	return s.db.Update(func(transaction *badger.Txn) error {
		entry := badger.NewEntry(composite(namespace, key), value)
		if ttl > 0 {
			entry = entry.WithTTL(ttl)
		}
		return transaction.SetEntry(entry)
	})
}

func (s *Store) Delete(namespace, key string) error {
	return s.db.Update(func(transaction *badger.Txn) error {
		return transaction.Delete(composite(namespace, key))
	})
}

// Add is first-writer-wins. Badger's serializable transaction records a missing
// key read, so concurrent calls cannot both commit with created=true.
func (s *Store) Add(
	namespace, key string,
	value []byte,
	ttl time.Duration,
) (bool, error) {
	created := false
	err := s.db.Update(func(transaction *badger.Txn) error {
		_, getErr := transaction.Get(composite(namespace, key))
		if getErr == nil {
			return nil
		}
		if !errors.Is(getErr, badger.ErrKeyNotFound) {
			return getErr
		}
		entry := badger.NewEntry(composite(namespace, key), value)
		if ttl > 0 {
			entry = entry.WithTTL(ttl)
		}
		created = true
		return transaction.SetEntry(entry)
	})
	if err != nil {
		return false, errors.WrapErr(err, "badgerkv: add")
	}
	return created, nil
}

func (s *Store) List(namespace string) ([]kv.Entry, error) {
	prefix := nsPrefix(namespace)
	var entries []kv.Entry
	err := s.db.View(func(transaction *badger.Txn) error {
		iterator := transaction.NewIterator(badger.DefaultIteratorOptions)
		defer iterator.Close()
		for iterator.Seek(prefix); iterator.ValidForPrefix(prefix); iterator.Next() {
			item := iterator.Item()
			value, err := item.ValueCopy(nil)
			if err != nil {
				return err
			}
			key := string(item.KeyCopy(nil)[len(prefix):])
			entries = append(entries, kv.Entry{Key: key, Value: value})
		}
		return nil
	})
	if err != nil {
		return nil, errors.WrapErr(err, "badgerkv: list")
	}
	return entries, nil
}
