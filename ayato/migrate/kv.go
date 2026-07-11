package migrate

import (
	"time"

	"github.com/Hayao0819/Kamisato/ayato/repository/kv"
)

// BulkSet batches when the backend supports it (keeping a large migration within a
// remote store's request budget), else falls back to per-key writes.
func BulkSet(s kv.Store, ns string, entries []kv.Entry, ttl time.Duration) error {
	if b, ok := s.(kv.BulkStore); ok {
		return b.BulkSet(ns, entries, ttl)
	}
	for _, e := range entries {
		if err := s.Set(ns, e.Key, e.Value, ttl); err != nil {
			return err
		}
	}
	return nil
}

func BulkDelete(s kv.Store, ns string, keys []string) error {
	if b, ok := s.(kv.BulkStore); ok {
		return b.BulkDelete(ns, keys)
	}
	for _, k := range keys {
		if err := s.Delete(ns, k); err != nil {
			return err
		}
	}
	return nil
}
