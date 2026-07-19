package migrate

import "github.com/Hayao0819/Kamisato/ayato/repository/kv"

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
