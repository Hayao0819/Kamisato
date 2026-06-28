// Package cfkv implements kv.Store on top of Cloudflare Workers KV.
//
// Workers KV is eventually consistent: a write may not be immediately visible on
// another edge node, and de-allowlisting an admin may briefly leave them able to
// act until the delete propagates. Deployments needing strict, immediate
// revocation should use the sql or badger backend instead.
package cfkv

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/Hayao0819/Kamisato/ayato/repository/kv"
	"github.com/Hayao0819/Kamisato/ayato/repository/kv/cfkv/logger"
	"github.com/Hayao0819/Kamisato/internal/utils"
	"github.com/cloudflare/cloudflare-go"
)

// sep separates the namespace from the key (see badgerkv for the rationale).
const sep = "\x00"

type Store struct {
	client      *cloudflare.API
	accountId   string
	namespaceId string
	ctx         context.Context
}

var _ kv.Store = (*Store)(nil)

func New(account, token, namespace string) (*Store, error) {
	c, err := cloudflare.NewWithAPIToken(token, cloudflare.UsingLogger(logger.Default()))
	if err != nil {
		return nil, utils.WrapErr(err, "cfkv: new client")
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

func composite(ns, key string) string { return ns + sep + key }

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
		return nil, utils.WrapErr(err, "cfkv: get")
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
		return utils.WrapErr(err, "cfkv: set")
	}
	_, err := s.client.WriteWorkersKVEntries(s.ctx, s.accountIdentifier(), cloudflare.WriteWorkersKVEntriesParams{
		NamespaceID: s.namespaceId,
		KVs: []*cloudflare.WorkersKVPair{{
			Key:           composite(ns, key),
			Value:         string(value),
			ExpirationTTL: int(ttl.Seconds()),
		}},
	})
	return utils.WrapErr(err, "cfkv: set with ttl")
}

func (s *Store) Delete(ns, key string) error {
	_, err := s.client.DeleteWorkersKVEntry(s.ctx, s.accountIdentifier(), cloudflare.DeleteWorkersKVEntryParams{
		NamespaceID: s.namespaceId,
		Key:         composite(ns, key),
	})
	return utils.WrapErr(err, "cfkv: delete")
}

// List pages through the Workers KV list-keys API by namespace prefix, fetching
// each value separately (the list API returns key names only). Cloudflare
// excludes expired keys from the listing.
func (s *Store) List(ns string) ([]kv.Entry, error) {
	prefix := ns + sep
	var out []kv.Entry
	cursor := ""
	for {
		resp, err := s.client.ListWorkersKVKeys(s.ctx, s.accountIdentifier(), cloudflare.ListWorkersKVsParams{
			NamespaceID: s.namespaceId,
			Prefix:      prefix,
			Cursor:      cursor,
		})
		if err != nil {
			return nil, utils.WrapErr(err, "cfkv: list keys")
		}
		for _, k := range resp.Result {
			if !strings.HasPrefix(k.Name, prefix) {
				continue
			}
			v, gerr := s.client.GetWorkersKV(s.ctx, s.accountIdentifier(), cloudflare.GetWorkersKVParams{
				NamespaceID: s.namespaceId,
				Key:         k.Name,
			})
			if gerr != nil {
				var notFound *cloudflare.NotFoundError
				if errors.As(gerr, &notFound) {
					continue
				}
				return nil, utils.WrapErr(gerr, "cfkv: list get value")
			}
			out = append(out, kv.Entry{Key: strings.TrimPrefix(k.Name, prefix), Value: v})
		}
		cursor = resp.ResultInfo.Cursor
		if cursor == "" {
			break
		}
	}
	return out, nil
}

// Close is a no-op: the Cloudflare client holds no long-lived resources.
func (s *Store) Close() error { return nil }
