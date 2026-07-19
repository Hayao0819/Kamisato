package cfkv

import (
	"encoding/base64"
	"fmt"
	"strings"

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
	keys, err := s.listRawKeys("")
	if err != nil {
		return nil, err
	}
	for _, key := range keys {
		if !isAppKey(key) {
			foreign = append(foreign, key)
		}
	}
	return foreign, nil
}

// DeleteRawKeys deletes by raw key name (bypassing composite), for pruning foreign keys.
func (s *Store) DeleteRawKeys(keys []string) error {
	if err := s.deleteKeys(keys); err != nil {
		return fmt.Errorf("cfkv: delete raw keys: %w", err)
	}
	return nil
}
