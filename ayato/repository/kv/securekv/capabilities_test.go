package securekv_test

import (
	"bytes"
	"testing"
	"time"

	"github.com/Hayao0819/Kamisato/ayato/repository/kv"
	"github.com/Hayao0819/Kamisato/ayato/repository/kv/securekv"
	"github.com/Hayao0819/Kamisato/internal/auth/secretbox"
)

type bulkAuditStore struct {
	kv.Store
	bulkSets    int
	bulkDeletes int
	foreignKeys []string
	rawDeletes  []string
}

func (store *bulkAuditStore) BulkSet(
	namespace string,
	entries []kv.Entry,
	ttl time.Duration,
) error {
	store.bulkSets++
	for _, entry := range entries {
		if err := store.Store.Set(namespace, entry.Key, entry.Value, ttl); err != nil {
			return err
		}
	}
	return nil
}

func (store *bulkAuditStore) BulkDelete(namespace string, keys []string) error {
	store.bulkDeletes++
	for _, key := range keys {
		if err := store.Store.Delete(namespace, key); err != nil {
			return err
		}
	}
	return nil
}

func (store *bulkAuditStore) ForeignKeys() ([]string, error) {
	return append([]string(nil), store.foreignKeys...), nil
}

func (store *bulkAuditStore) DeleteRawKeys(keys []string) error {
	store.rawDeletes = append(store.rawDeletes, keys...)
	return nil
}

type allCapabilitiesStore struct {
	*bulkAuditStore
	adder kv.Adder
}

func (store *allCapabilitiesStore) Add(
	namespace,
	key string,
	value []byte,
	ttl time.Duration,
) (bool, error) {
	return store.adder.Add(namespace, key, value, ttl)
}

func TestBulkAndAuditCapabilitiesPreserved(t *testing.T) {
	mem := newMem()
	backend := &bulkAuditStore{
		Store:       mem,
		foreignKeys: []string{"foreign/raw"},
	}
	store := securekv.New(backend, newBox(t), []string{allowNS})

	bulk, ok := store.(kv.BulkStore)
	if !ok {
		t.Fatal("wrapper must expose kv.BulkStore when the backend does")
	}
	auditor, ok := store.(kv.KeyAuditor)
	if !ok {
		t.Fatal("wrapper must expose kv.KeyAuditor when the backend does")
	}
	if _, ok := store.(kv.Adder); ok {
		t.Fatal("wrapper advertised kv.Adder although the backend does not")
	}

	entries := []kv.Entry{{Key: "42", Value: []byte("octocat")}}
	if err := bulk.BulkSet(allowNS, entries, time.Hour); err != nil {
		t.Fatalf("BulkSet: %v", err)
	}
	if backend.bulkSets != 1 {
		t.Fatalf("backend BulkSet calls = %d, want 1", backend.bulkSets)
	}
	if string(entries[0].Value) != "octocat" {
		t.Fatal("BulkSet mutated the caller's entry")
	}
	raw := mem.raw(allowNS, "42")
	if !secretbox.IsSealed(raw) || bytes.Contains(raw, []byte("octocat")) {
		t.Fatal("BulkSet did not encrypt the sensitive value at rest")
	}
	if got, err := store.Get(allowNS, "42"); err != nil || string(got) != "octocat" {
		t.Fatalf("Get after BulkSet = (%q, %v)", got, err)
	}

	if err := bulk.BulkDelete(allowNS, []string{"42"}); err != nil {
		t.Fatalf("BulkDelete: %v", err)
	}
	if backend.bulkDeletes != 1 {
		t.Fatalf("backend BulkDelete calls = %d, want 1", backend.bulkDeletes)
	}
	keys, err := auditor.ForeignKeys()
	if err != nil || len(keys) != 1 || keys[0] != "foreign/raw" {
		t.Fatalf("ForeignKeys = (%v, %v)", keys, err)
	}
	if err := auditor.DeleteRawKeys([]string{"foreign/raw"}); err != nil {
		t.Fatalf("DeleteRawKeys: %v", err)
	}
	if len(backend.rawDeletes) != 1 || backend.rawDeletes[0] != "foreign/raw" {
		t.Fatalf("raw deletes = %v", backend.rawDeletes)
	}
}

func TestWrapperAdvertisesExactlyBackendCapabilities(t *testing.T) {
	mem := newMem()
	box := newBox(t)

	plain := securekv.New(struct{ kv.Store }{Store: mem}, box, []string{allowNS})
	if _, ok := plain.(kv.Adder); ok {
		t.Error("plain backend gained Adder")
	}
	if _, ok := plain.(kv.BulkStore); ok {
		t.Error("plain backend gained BulkStore")
	}
	if _, ok := plain.(kv.KeyAuditor); ok {
		t.Error("plain backend gained KeyAuditor")
	}

	backend := &bulkAuditStore{Store: mem}
	full := securekv.New(
		&allCapabilitiesStore{bulkAuditStore: backend, adder: mem},
		box,
		[]string{allowNS},
	)
	if _, ok := full.(kv.Adder); !ok {
		t.Error("full backend lost Adder")
	}
	if _, ok := full.(kv.BulkStore); !ok {
		t.Error("full backend lost BulkStore")
	}
	if _, ok := full.(kv.KeyAuditor); !ok {
		t.Error("full backend lost KeyAuditor")
	}
}
