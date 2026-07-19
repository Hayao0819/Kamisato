// Package securekv decorates a kv.Store so values in designated (sensitive)
// namespaces are encrypted at rest via a secretbox.SecretBox, while every other
// namespace passes through untouched. Reads transparently accept a still-plaintext
// value written before encryption was enabled, so turning encryption on does not
// strand existing data.
package securekv

import (
	"slices"
	"time"

	"github.com/Hayao0819/Kamisato/ayato/repository/kv"
	"github.com/Hayao0819/Kamisato/internal/auth/secretbox"
	"github.com/Hayao0819/Kamisato/internal/errors"
)

type store struct {
	inner     kv.Store
	box       secretbox.SecretBox
	encrypted map[string]struct{}
}

// New wraps inner so writes to any namespace in encryptedNamespaces are sealed
// with box and reads are opened. With a nil box or no namespaces it returns inner
// unchanged, so encryption stays fully opt-in.
func New(inner kv.Store, box secretbox.SecretBox, encryptedNamespaces []string) kv.Store {
	if box == nil || len(encryptedNamespaces) == 0 {
		return inner
	}
	enc := make(map[string]struct{}, len(encryptedNamespaces))
	for _, ns := range encryptedNamespaces {
		enc[ns] = struct{}{}
	}
	s := &store{inner: inner, box: box, encrypted: enc}
	capabilities := 0
	if _, ok := inner.(kv.Adder); ok {
		capabilities |= capabilityAdder
	}
	if _, ok := inner.(kv.BulkStore); ok {
		capabilities |= capabilityBulk
	}
	if _, ok := inner.(kv.KeyAuditor); ok {
		capabilities |= capabilityAuditor
	}
	switch capabilities {
	case capabilityAdder:
		return &adderStore{store: s}
	case capabilityBulk:
		return &bulkStore{store: s}
	case capabilityAuditor:
		return &auditorStore{store: s}
	case capabilityAdder | capabilityBulk:
		return &adderBulkStore{store: s}
	case capabilityAdder | capabilityAuditor:
		return &adderAuditorStore{store: s}
	case capabilityBulk | capabilityAuditor:
		return &bulkAuditorStore{store: s}
	case capabilityAdder | capabilityBulk | capabilityAuditor:
		return &fullStore{store: s}
	default:
		return s
	}
}

const (
	capabilityAdder = 1 << iota
	capabilityBulk
	capabilityAuditor
)

func (s *store) encrypts(ns string) bool {
	_, ok := s.encrypted[ns]
	return ok
}

func (s *store) Get(ns, key string) ([]byte, error) {
	v, err := s.inner.Get(ns, key)
	if err != nil {
		return nil, err
	}
	if !s.encrypts(ns) {
		return v, nil
	}
	return s.open(v)
}

func (s *store) Set(ns, key string, value []byte, ttl time.Duration) error {
	value, err := s.seal(ns, value)
	if err != nil {
		return err
	}
	return s.inner.Set(ns, key, value, ttl)
}

func (s *store) Delete(ns, key string) error { return s.inner.Delete(ns, key) }

func (s *store) List(ns string) ([]kv.Entry, error) {
	entries, err := s.inner.List(ns)
	if err != nil || !s.encrypts(ns) {
		return entries, err
	}
	for i := range entries {
		plain, oerr := s.open(entries[i].Value)
		if oerr != nil {
			return nil, oerr
		}
		entries[i].Value = plain
	}
	return entries, nil
}

func (s *store) Close() error { return s.inner.Close() }

func (s *store) seal(ns string, value []byte) ([]byte, error) {
	if !s.encrypts(ns) {
		return value, nil
	}
	sealed, err := s.box.Seal(value)
	if err != nil {
		return nil, errors.WrapErr(err, "securekv: seal")
	}
	return sealed, nil
}

// open decrypts a sealed value, but returns a pre-encryption plaintext value
// unchanged — the transparent migration path when encryption is turned on over an
// existing store.
func (s *store) open(v []byte) ([]byte, error) {
	if !secretbox.IsSealed(v) {
		return v, nil
	}
	plain, err := s.box.Open(v)
	if err != nil {
		return nil, errors.WrapErr(err, "securekv: open")
	}
	return plain, nil
}

func (s *store) add(ns, key string, value []byte, ttl time.Duration) (bool, error) {
	value, err := s.seal(ns, value)
	if err != nil {
		return false, err
	}
	return s.inner.(kv.Adder).Add(ns, key, value, ttl)
}

func (s *store) bulkSet(ns string, entries []kv.Entry, ttl time.Duration) error {
	if !s.encrypts(ns) {
		return s.inner.(kv.BulkStore).BulkSet(ns, entries, ttl)
	}
	sealedEntries := slices.Clone(entries)
	for i := range sealedEntries {
		sealed, err := s.seal(ns, entries[i].Value)
		if err != nil {
			return err
		}
		sealedEntries[i].Value = sealed
	}
	return s.inner.(kv.BulkStore).BulkSet(ns, sealedEntries, ttl)
}

func (s *store) bulkDelete(ns string, keys []string) error {
	return s.inner.(kv.BulkStore).BulkDelete(ns, keys)
}

func (s *store) foreignKeys() ([]string, error) {
	return s.inner.(kv.KeyAuditor).ForeignKeys()
}

func (s *store) deleteRawKeys(keys []string) error {
	return s.inner.(kv.KeyAuditor).DeleteRawKeys(keys)
}

// Each wrapper below advertises exactly the optional interfaces implemented by
// the backend. Returning one universal wrapper would make an unsupported
// capability appear available to callers that select behavior by type assertion.
type adderStore struct {
	*store
}

var _ kv.Adder = (*adderStore)(nil)

func (s *adderStore) Add(ns, key string, value []byte, ttl time.Duration) (bool, error) {
	return s.add(ns, key, value, ttl)
}

type bulkStore struct {
	*store
}

var _ kv.BulkStore = (*bulkStore)(nil)

func (s *bulkStore) BulkSet(ns string, entries []kv.Entry, ttl time.Duration) error {
	return s.bulkSet(ns, entries, ttl)
}

func (s *bulkStore) BulkDelete(ns string, keys []string) error {
	return s.bulkDelete(ns, keys)
}

type auditorStore struct {
	*store
}

var _ kv.KeyAuditor = (*auditorStore)(nil)

func (s *auditorStore) ForeignKeys() ([]string, error) {
	return s.foreignKeys()
}

func (s *auditorStore) DeleteRawKeys(keys []string) error {
	return s.deleteRawKeys(keys)
}

type adderBulkStore struct {
	*store
}

var (
	_ kv.Adder     = (*adderBulkStore)(nil)
	_ kv.BulkStore = (*adderBulkStore)(nil)
)

func (s *adderBulkStore) Add(ns, key string, value []byte, ttl time.Duration) (bool, error) {
	return s.add(ns, key, value, ttl)
}

func (s *adderBulkStore) BulkSet(ns string, entries []kv.Entry, ttl time.Duration) error {
	return s.bulkSet(ns, entries, ttl)
}

func (s *adderBulkStore) BulkDelete(ns string, keys []string) error {
	return s.bulkDelete(ns, keys)
}

type adderAuditorStore struct {
	*store
}

var (
	_ kv.Adder      = (*adderAuditorStore)(nil)
	_ kv.KeyAuditor = (*adderAuditorStore)(nil)
)

func (s *adderAuditorStore) Add(ns, key string, value []byte, ttl time.Duration) (bool, error) {
	return s.add(ns, key, value, ttl)
}

func (s *adderAuditorStore) ForeignKeys() ([]string, error) {
	return s.foreignKeys()
}

func (s *adderAuditorStore) DeleteRawKeys(keys []string) error {
	return s.deleteRawKeys(keys)
}

type bulkAuditorStore struct {
	*store
}

var (
	_ kv.BulkStore  = (*bulkAuditorStore)(nil)
	_ kv.KeyAuditor = (*bulkAuditorStore)(nil)
)

func (s *bulkAuditorStore) BulkSet(ns string, entries []kv.Entry, ttl time.Duration) error {
	return s.bulkSet(ns, entries, ttl)
}

func (s *bulkAuditorStore) BulkDelete(ns string, keys []string) error {
	return s.bulkDelete(ns, keys)
}

func (s *bulkAuditorStore) ForeignKeys() ([]string, error) {
	return s.foreignKeys()
}

func (s *bulkAuditorStore) DeleteRawKeys(keys []string) error {
	return s.deleteRawKeys(keys)
}

type fullStore struct {
	*store
}

var (
	_ kv.Adder      = (*fullStore)(nil)
	_ kv.BulkStore  = (*fullStore)(nil)
	_ kv.KeyAuditor = (*fullStore)(nil)
)

func (s *fullStore) Add(ns, key string, value []byte, ttl time.Duration) (bool, error) {
	return s.add(ns, key, value, ttl)
}

func (s *fullStore) BulkSet(ns string, entries []kv.Entry, ttl time.Duration) error {
	return s.bulkSet(ns, entries, ttl)
}

func (s *fullStore) BulkDelete(ns string, keys []string) error {
	return s.bulkDelete(ns, keys)
}

func (s *fullStore) ForeignKeys() ([]string, error) {
	return s.foreignKeys()
}

func (s *fullStore) DeleteRawKeys(keys []string) error {
	return s.deleteRawKeys(keys)
}
