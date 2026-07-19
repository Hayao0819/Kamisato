package securekv_test

import (
	"sync"
	"testing"
	"time"

	"filippo.io/age"

	"github.com/Hayao0819/Kamisato/ayato/repository/kv"
	"github.com/Hayao0819/Kamisato/ayato/repository/kv/schema"
	"github.com/Hayao0819/Kamisato/internal/auth/secretbox"
)

type memStore struct {
	mu     sync.Mutex
	values map[string]map[string][]byte
}

func newMem() *memStore {
	return &memStore{values: map[string]map[string][]byte{}}
}

func (store *memStore) raw(namespace, key string) []byte {
	store.mu.Lock()
	defer store.mu.Unlock()
	return store.values[namespace][key]
}

func (store *memStore) Get(namespace, key string) ([]byte, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	value, ok := store.values[namespace][key]
	if !ok {
		return nil, kv.ErrNotFound
	}
	return append([]byte(nil), value...), nil
}

func (store *memStore) Set(
	namespace,
	key string,
	value []byte,
	_ time.Duration,
) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.values[namespace] == nil {
		store.values[namespace] = map[string][]byte{}
	}
	store.values[namespace][key] = append([]byte(nil), value...)
	return nil
}

func (store *memStore) Delete(namespace, key string) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	delete(store.values[namespace], key)
	return nil
}

func (store *memStore) List(namespace string) ([]kv.Entry, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	entries := make([]kv.Entry, 0, len(store.values[namespace]))
	for key, value := range store.values[namespace] {
		entries = append(entries, kv.Entry{
			Key: key, Value: append([]byte(nil), value...),
		})
	}
	return entries, nil
}

func (store *memStore) Add(
	namespace,
	key string,
	value []byte,
	_ time.Duration,
) (bool, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.values[namespace] == nil {
		store.values[namespace] = map[string][]byte{}
	}
	if _, ok := store.values[namespace][key]; ok {
		return false, nil
	}
	store.values[namespace][key] = append([]byte(nil), value...)
	return true, nil
}

func (store *memStore) Close() error { return nil }

func newBox(t *testing.T) secretbox.SecretBox {
	t.Helper()
	identity, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatalf("generate identity: %v", err)
	}
	box, err := secretbox.NewAgeX25519(identity.String())
	if err != nil {
		t.Fatalf("NewAgeX25519: %v", err)
	}
	return box
}

const allowNS = schema.AdminAllowlist
