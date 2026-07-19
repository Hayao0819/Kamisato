package cfkv

import (
	"fmt"
	"time"

	"github.com/cloudflare/cloudflare-go"

	"github.com/Hayao0819/Kamisato/ayato/repository/kv"
)

var _ kv.BulkStore = (*Store)(nil)

// Workers KV bulk write/delete cap each request at 10,000 keys.
const bulkChunk = 10000

func (s *Store) BulkSet(ns string, entries []kv.Entry, ttl time.Duration) error {
	pairs := make([]*cloudflare.WorkersKVPair, 0, len(entries))
	for _, e := range entries {
		p := &cloudflare.WorkersKVPair{Key: composite(ns, e.Key), Value: string(e.Value)}
		if ttl > 0 {
			p.ExpirationTTL = int(ttl.Seconds())
		}
		pairs = append(pairs, p)
	}
	err := forChunks(pairs, func(chunk []*cloudflare.WorkersKVPair) error {
		_, err := s.client.WriteWorkersKVEntries(s.ctx, s.accountIdentifier(), cloudflare.WriteWorkersKVEntriesParams{
			NamespaceID: s.namespaceId,
			KVs:         chunk,
		})
		return err
	})
	if err != nil {
		return fmt.Errorf("cfkv: bulk set: %w", err)
	}
	return nil
}

func (s *Store) BulkDelete(ns string, keys []string) error {
	enc := make([]string, len(keys))
	for i, k := range keys {
		enc[i] = composite(ns, k)
	}
	if err := s.deleteKeys(enc); err != nil {
		return fmt.Errorf("cfkv: bulk delete: %w", err)
	}
	return nil
}

func (s *Store) deleteKeys(keys []string) error {
	return forChunks(keys, func(chunk []string) error {
		_, err := s.client.DeleteWorkersKVEntries(s.ctx, s.accountIdentifier(), cloudflare.DeleteWorkersKVEntriesParams{
			NamespaceID: s.namespaceId,
			Keys:        chunk,
		})
		return err
	})
}

func forChunks[T any](items []T, apply func([]T) error) error {
	for start := 0; start < len(items); start += bulkChunk {
		if err := apply(items[start:min(start+bulkChunk, len(items))]); err != nil {
			return err
		}
	}
	return nil
}
