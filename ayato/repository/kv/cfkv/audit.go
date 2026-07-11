package cfkv

import (
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/cloudflare/cloudflare-go"

	"github.com/Hayao0819/Kamisato/ayato/repository/kv"
)

var _ kv.KeyAuditor = (*Store)(nil)

// isAppKey reports whether raw has the composite() shape this backend writes:
// base64url(ns) "." base64url(key). A key injected by hand through the dashboard
// does not, so anything failing this is foreign.
func isAppKey(raw string) bool {
	parts := strings.Split(raw, sep)
	if len(parts) != 2 {
		return false
	}
	for _, p := range parts {
		if p == "" {
			return false
		}
		if _, err := base64.RawURLEncoding.DecodeString(p); err != nil {
			return false
		}
	}
	return true
}

// ForeignKeys lists every key in the namespace and returns those not in composite form.
func (s *Store) ForeignKeys() ([]string, error) {
	var foreign []string
	cursor := ""
	for {
		resp, err := s.client.ListWorkersKVKeys(s.ctx, s.accountIdentifier(), cloudflare.ListWorkersKVsParams{
			NamespaceID: s.namespaceId,
			Cursor:      cursor,
		})
		if err != nil {
			return nil, fmt.Errorf("cfkv: list keys: %w", err)
		}
		for _, k := range resp.Result {
			if !isAppKey(k.Name) {
				foreign = append(foreign, k.Name)
			}
		}
		cursor = resp.ResultInfo.Cursor
		if cursor == "" {
			break
		}
	}
	return foreign, nil
}

// DeleteRawKeys deletes by raw key name (bypassing composite), for pruning foreign keys.
func (s *Store) DeleteRawKeys(keys []string) error {
	for start := 0; start < len(keys); start += bulkChunk {
		end := min(start+bulkChunk, len(keys))
		if _, err := s.client.DeleteWorkersKVEntries(s.ctx, s.accountIdentifier(), cloudflare.DeleteWorkersKVEntriesParams{
			NamespaceID: s.namespaceId,
			Keys:        keys[start:end],
		}); err != nil {
			return fmt.Errorf("cfkv: delete raw keys: %w", err)
		}
	}
	return nil
}
