// Package securekv decorates a kv.Store so values in designated (sensitive)
// namespaces are encrypted at rest via a secretbox.SecretBox, while every other
// namespace passes through untouched. Reads transparently accept a still-plaintext
// value written before encryption was enabled, so turning encryption on does not
// strand existing data.
package securekv

import (
	"time"

	"github.com/Hayao0819/Kamisato/ayato/repository/kv"
	"github.com/Hayao0819/Kamisato/internal/errwrap"
	"github.com/Hayao0819/Kamisato/internal/secretbox"
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
	// Preserve the atomic set-if-absent capability when the backend offers it, so
	// the replay and rate-limit guards keep their race-free path. Those namespaces
	// are never encrypted, so Add just delegates.
	if _, ok := inner.(kv.Adder); ok {
		return &adderStore{store: s}
	}
	return s
}

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
	if s.encrypts(ns) {
		sealed, err := s.box.Seal(value)
		if err != nil {
			return errwrap.WrapErr(err, "securekv: seal")
		}
		value = sealed
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

// open decrypts a sealed value, but returns a pre-encryption plaintext value
// unchanged — the transparent migration path when encryption is turned on over an
// existing store.
func (s *store) open(v []byte) ([]byte, error) {
	if !secretbox.IsSealed(v) {
		return v, nil
	}
	plain, err := s.box.Open(v)
	if err != nil {
		return nil, errwrap.WrapErr(err, "securekv: open")
	}
	return plain, nil
}

// adderStore re-exposes the backend's atomic Add for callers that type-assert
// kv.Adder. An encrypted namespace seals the value before delegating.
type adderStore struct {
	*store
}

var _ kv.Adder = (*adderStore)(nil)

func (s *adderStore) Add(ns, key string, value []byte, ttl time.Duration) (bool, error) {
	if s.encrypts(ns) {
		sealed, err := s.box.Seal(value)
		if err != nil {
			return false, errwrap.WrapErr(err, "securekv: seal")
		}
		value = sealed
	}
	return s.inner.(kv.Adder).Add(ns, key, value, ttl)
}
