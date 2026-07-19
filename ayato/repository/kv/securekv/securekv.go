// Package securekv decorates a kv.Store with encryption at rest for selected
// namespaces. Other namespaces and optional backend capabilities pass through.
package securekv

import (
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

// New returns inner unchanged when encryption is disabled. Otherwise the
// decorator advertises exactly the optional capabilities inner provides.
func New(
	inner kv.Store,
	box secretbox.SecretBox,
	encryptedNamespaces []string,
) kv.Store {
	if box == nil || len(encryptedNamespaces) == 0 {
		return inner
	}
	encrypted := make(map[string]struct{}, len(encryptedNamespaces))
	for _, namespace := range encryptedNamespaces {
		encrypted[namespace] = struct{}{}
	}
	core := &store{
		inner:     inner,
		box:       box,
		encrypted: encrypted,
	}
	return wrapCapabilities(core)
}

func (s *store) encrypts(namespace string) bool {
	_, encrypted := s.encrypted[namespace]
	return encrypted
}

func (s *store) Get(namespace, key string) ([]byte, error) {
	value, err := s.inner.Get(namespace, key)
	if err != nil {
		return nil, err
	}
	if !s.encrypts(namespace) {
		return value, nil
	}
	return s.open(value)
}

func (s *store) Set(
	namespace, key string,
	value []byte,
	ttl time.Duration,
) error {
	value, err := s.seal(namespace, value)
	if err != nil {
		return err
	}
	return s.inner.Set(namespace, key, value, ttl)
}

func (s *store) Delete(namespace, key string) error {
	return s.inner.Delete(namespace, key)
}

func (s *store) List(namespace string) ([]kv.Entry, error) {
	entries, err := s.inner.List(namespace)
	if err != nil || !s.encrypts(namespace) {
		return entries, err
	}
	for index := range entries {
		plain, err := s.open(entries[index].Value)
		if err != nil {
			return nil, err
		}
		entries[index].Value = plain
	}
	return entries, nil
}

func (s *store) Close() error {
	return s.inner.Close()
}

func (s *store) seal(namespace string, value []byte) ([]byte, error) {
	if !s.encrypts(namespace) {
		return value, nil
	}
	sealed, err := s.box.Seal(value)
	if err != nil {
		return nil, errors.WrapErr(err, "securekv: seal")
	}
	return sealed, nil
}

// open accepts plaintext written before encryption was enabled. This provides a
// transparent read path while a deployment migrates existing values.
func (s *store) open(value []byte) ([]byte, error) {
	if !secretbox.IsSealed(value) {
		return value, nil
	}
	plain, err := s.box.Open(value)
	if err != nil {
		return nil, errors.WrapErr(err, "securekv: open")
	}
	return plain, nil
}
