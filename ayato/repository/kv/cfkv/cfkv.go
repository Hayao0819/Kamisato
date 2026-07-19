// Package cfkv implements kv.Store on top of Cloudflare Workers KV.
//
// Workers KV is eventually consistent: a write may not be immediately visible on
// another edge node, and de-allowlisting an admin may briefly leave them able to
// act until the delete propagates. Deployments needing strict, immediate
// revocation should use the sql or badger backend instead.
package cfkv

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"github.com/Hayao0819/Kamisato/internal/errors"

	"github.com/cloudflare/cloudflare-go"

	"github.com/Hayao0819/Kamisato/ayato/repository/kv"
	"github.com/Hayao0819/Kamisato/ayato/repository/kv/slogadapter"
)

// Keys are URL-safe base64 so no byte can break the Workers KV REST URL: the NUL
// separator badgerkv uses would become %00, which Cloudflare's edge rejects with a
// 400. ns and key are encoded separately and joined by "." (not in the base64url
// alphabet) so List still matches by prefix.
const sep = "."

type Store struct {
	client      *cloudflare.API
	accountId   string
	namespaceId string
	ctx         context.Context
}

var _ kv.Store = (*Store)(nil)

func New(account, token, namespace string) (*Store, error) {
	c, err := cloudflare.NewWithAPIToken(token, cloudflare.UsingLogger(slogadapter.Default()))
	if err != nil {
		return nil, fmt.Errorf("cfkv: new client: %w", err)
	}
	return &Store{
		client:      c,
		accountId:   account,
		namespaceId: namespace,
		ctx:         context.Background(),
	}, nil
}

func (s *Store) accountIdentifier() *cloudflare.ResourceContainer {
	return cloudflare.AccountIdentifier(s.accountId)
}

func encPart(s string) string { return base64.RawURLEncoding.EncodeToString([]byte(s)) }

func nsPrefix(ns string) string { return encPart(ns) + sep }

func composite(ns, key string) string { return encPart(ns) + sep + encPart(key) }

// A Cloudflare not-found is mapped to kv.ErrNotFound so misses surface uniformly.
func (s *Store) Get(ns, key string) ([]byte, error) {
	v, err := s.client.GetWorkersKV(s.ctx, s.accountIdentifier(), cloudflare.GetWorkersKVParams{
		NamespaceID: s.namespaceId,
		Key:         composite(ns, key),
	})
	if err != nil {
		var notFound *cloudflare.NotFoundError
		if errors.As(err, &notFound) {
			return nil, kv.ErrNotFound
		}
		return nil, fmt.Errorf("cfkv: get: %w", err)
	}
	return v, nil
}

// A positive ttl uses Workers KV's native expiration_ttl, which only the
// bulk-write API carries; ttl == 0 writes without expiry. Cloudflare rejects a
// TTL below 60s, so use minute-scale or no expiry.
func (s *Store) Set(ns, key string, value []byte, ttl time.Duration) error {
	if ttl <= 0 {
		_, err := s.client.WriteWorkersKVEntry(s.ctx, s.accountIdentifier(), cloudflare.WriteWorkersKVEntryParams{
			NamespaceID: s.namespaceId,
			Key:         composite(ns, key),
			Value:       value,
		})
		if err != nil {
			return fmt.Errorf("cfkv: set: %w", err)
		}
		return nil
	}
	_, err := s.client.WriteWorkersKVEntries(s.ctx, s.accountIdentifier(), cloudflare.WriteWorkersKVEntriesParams{
		NamespaceID: s.namespaceId,
		KVs: []*cloudflare.WorkersKVPair{{
			Key:           composite(ns, key),
			Value:         string(value),
			ExpirationTTL: int(ttl.Seconds()),
		}},
	})
	if err != nil {
		return fmt.Errorf("cfkv: set with ttl: %w", err)
	}
	return nil
}

func (s *Store) Delete(ns, key string) error {
	_, err := s.client.DeleteWorkersKVEntry(s.ctx, s.accountIdentifier(), cloudflare.DeleteWorkersKVEntryParams{
		NamespaceID: s.namespaceId,
		Key:         composite(ns, key),
	})
	if err != nil {
		return fmt.Errorf("cfkv: delete: %w", err)
	}
	return nil
}

// List pages through the Workers KV list-keys API by namespace prefix, fetching
// each value separately (the list API returns key names only). Cloudflare
// excludes expired keys from the listing.
func (s *Store) List(ns string) ([]kv.Entry, error) {
	prefix := nsPrefix(ns)
	var out []kv.Entry
	keys, err := s.listRawKeys(prefix)
	if err != nil {
		return nil, err
	}
	for _, rawKey := range keys {
		v, getErr := s.client.GetWorkersKV(s.ctx, s.accountIdentifier(), cloudflare.GetWorkersKVParams{
			NamespaceID: s.namespaceId,
			Key:         rawKey,
		})
		if getErr != nil {
			var notFound *cloudflare.NotFoundError
			if errors.As(getErr, &notFound) {
				continue
			}
			return nil, fmt.Errorf("cfkv: list get value: %w", getErr)
		}
		key, decodeErr := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(rawKey, prefix))
		if decodeErr != nil {
			continue
		}
		out = append(out, kv.Entry{Key: string(key), Value: v})
	}
	return out, nil
}

func (s *Store) listRawKeys(prefix string) ([]string, error) {
	var keys []string
	cursor := ""
	for {
		resp, err := s.client.ListWorkersKVKeys(s.ctx, s.accountIdentifier(), cloudflare.ListWorkersKVsParams{
			NamespaceID: s.namespaceId,
			Prefix:      prefix,
			Cursor:      cursor,
		})
		if err != nil {
			return nil, fmt.Errorf("cfkv: list keys: %w", err)
		}
		for _, key := range resp.Result {
			if prefix != "" && !strings.HasPrefix(key.Name, prefix) {
				continue
			}
			keys = append(keys, key.Name)
		}
		cursor = resp.Cursor
		if cursor == "" {
			break
		}
	}
	return keys, nil
}

// Close is a no-op: the Cloudflare client holds no long-lived resources.
func (s *Store) Close() error { return nil }
