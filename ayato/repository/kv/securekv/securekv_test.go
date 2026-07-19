package securekv_test

import (
	"bytes"
	"sync"
	"testing"
	"time"

	"filippo.io/age"

	"github.com/Hayao0819/Kamisato/ayato/repository/kv"
	"github.com/Hayao0819/Kamisato/ayato/repository/kv/schema"
	"github.com/Hayao0819/Kamisato/ayato/repository/kv/securekv"
	"github.com/Hayao0819/Kamisato/internal/auth/secretbox"
)

// memStore is a minimal in-memory kv.Store (with Adder) for exercising the
// decorator without a real backend. It also lets a test inspect the raw bytes a
// namespace holds, to prove encryption actually happened at rest.
type memStore struct {
	mu sync.Mutex
	m  map[string]map[string][]byte
}

func newMem() *memStore { return &memStore{m: map[string]map[string][]byte{}} }

func (s *memStore) raw(ns, key string) []byte {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.m[ns][key]
}

func (s *memStore) Get(ns, key string) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.m[ns][key]
	if !ok {
		return nil, kv.ErrNotFound
	}
	return append([]byte(nil), v...), nil
}

func (s *memStore) Set(ns, key string, value []byte, _ time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.m[ns] == nil {
		s.m[ns] = map[string][]byte{}
	}
	s.m[ns][key] = append([]byte(nil), value...)
	return nil
}

func (s *memStore) Delete(ns, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.m[ns], key)
	return nil
}

func (s *memStore) List(ns string) ([]kv.Entry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []kv.Entry
	for k, v := range s.m[ns] {
		out = append(out, kv.Entry{Key: k, Value: append([]byte(nil), v...)})
	}
	return out, nil
}

func (s *memStore) Add(ns, key string, value []byte, _ time.Duration) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.m[ns] == nil {
		s.m[ns] = map[string][]byte{}
	}
	if _, ok := s.m[ns][key]; ok {
		return false, nil
	}
	s.m[ns][key] = append([]byte(nil), value...)
	return true, nil
}

func (s *memStore) Close() error { return nil }

func newBox(t *testing.T) secretbox.SecretBox {
	t.Helper()
	id, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatalf("generate identity: %v", err)
	}
	box, err := secretbox.NewAgeX25519(id.String())
	if err != nil {
		t.Fatalf("NewAgeX25519: %v", err)
	}
	return box
}

const allowNS = schema.AdminAllowlist

// An entry in an encrypted namespace persists as ciphertext and reads back as the
// original plaintext.
func TestEncryptedNamespaceRoundTrip(t *testing.T) {
	mem := newMem()
	s := securekv.New(mem, newBox(t), []string{allowNS})

	if err := s.Set(allowNS, "42", []byte("octocat"), 0); err != nil {
		t.Fatalf("Set: %v", err)
	}

	// At rest the value is an age ciphertext, not the login.
	stored := mem.raw(allowNS, "42")
	if bytes.Contains(stored, []byte("octocat")) {
		t.Fatal("value must be encrypted at rest, not stored as plaintext")
	}
	if !secretbox.IsSealed(stored) {
		t.Fatal("stored value must be an age ciphertext")
	}

	got, err := s.Get(allowNS, "42")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if string(got) != "octocat" {
		t.Fatalf("Get = %q, want octocat", got)
	}

	entries, err := s.List(allowNS)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(entries) != 1 || entries[0].Key != "42" || string(entries[0].Value) != "octocat" {
		t.Fatalf("List = %+v, want one decrypted entry", entries)
	}
}

// A value written before encryption was enabled (plaintext already on disk) must
// still read back — the transparent migration path.
func TestReadsPreEncryptionPlaintext(t *testing.T) {
	mem := newMem()
	// Simulate the pre-encryption store: plaintext written directly to the backend.
	if err := mem.Set(allowNS, "7", []byte("legacy"), 0); err != nil {
		t.Fatal(err)
	}

	s := securekv.New(mem, newBox(t), []string{allowNS})
	got, err := s.Get(allowNS, "7")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if string(got) != "legacy" {
		t.Fatalf("Get = %q, want legacy (migration read)", got)
	}

	entries, err := s.List(allowNS)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(entries) != 1 || string(entries[0].Value) != "legacy" {
		t.Fatalf("List = %+v, want the legacy plaintext", entries)
	}
}

// A namespace outside the encrypted set is stored verbatim.
func TestUnencryptedNamespaceIsPlaintext(t *testing.T) {
	mem := newMem()
	s := securekv.New(mem, newBox(t), []string{allowNS})

	if err := s.Set("replay", "nonce", []byte("1"), 0); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if got := mem.raw("replay", "nonce"); string(got) != "1" {
		t.Fatalf("unencrypted namespace stored %q, want plaintext 1", got)
	}
}

// With no box the decorator is a pass-through, so encryption stays opt-in.
func TestDisabledIsPassthrough(t *testing.T) {
	mem := newMem()
	s := securekv.New(mem, nil, []string{allowNS})
	if s != kv.Store(mem) {
		t.Fatal("a nil box must return the inner store unchanged")
	}
}

// The atomic Adder survives the wrapper (needed by the replay/rate-limit guards),
// and Add on an encrypted namespace still seals.
func TestAdderPreservedAndSeals(t *testing.T) {
	mem := newMem()
	s := securekv.New(mem, newBox(t), []string{allowNS})

	adder, ok := s.(kv.Adder)
	if !ok {
		t.Fatal("wrapper must expose kv.Adder when the backend does")
	}

	created, err := adder.Add("replay", "id", []byte("1"), time.Minute)
	if err != nil || !created {
		t.Fatalf("first Add = (%v, %v), want (true, nil)", created, err)
	}
	created, err = adder.Add("replay", "id", []byte("1"), time.Minute)
	if err != nil || created {
		t.Fatalf("second Add = (%v, %v), want (false, nil)", created, err)
	}

	if _, err := adder.Add(allowNS, "9", []byte("mona"), 0); err != nil {
		t.Fatalf("Add encrypted: %v", err)
	}
	if bytes.Contains(mem.raw(allowNS, "9"), []byte("mona")) {
		t.Fatal("Add on an encrypted namespace must seal the value")
	}
	got, err := s.Get(allowNS, "9")
	if err != nil || string(got) != "mona" {
		t.Fatalf("Get after encrypted Add = (%q, %v)", got, err)
	}
}

type bulkAuditStore struct {
	kv.Store
	bulkSets    int
	bulkDeletes int
	foreignKeys []string
	rawDeletes  []string
}

func (s *bulkAuditStore) BulkSet(ns string, entries []kv.Entry, ttl time.Duration) error {
	s.bulkSets++
	for _, entry := range entries {
		if err := s.Store.Set(ns, entry.Key, entry.Value, ttl); err != nil {
			return err
		}
	}
	return nil
}

func (s *bulkAuditStore) BulkDelete(ns string, keys []string) error {
	s.bulkDeletes++
	for _, key := range keys {
		if err := s.Store.Delete(ns, key); err != nil {
			return err
		}
	}
	return nil
}

func (s *bulkAuditStore) ForeignKeys() ([]string, error) {
	return append([]string(nil), s.foreignKeys...), nil
}

func (s *bulkAuditStore) DeleteRawKeys(keys []string) error {
	s.rawDeletes = append(s.rawDeletes, keys...)
	return nil
}

type allCapabilitiesStore struct {
	*bulkAuditStore
	adder kv.Adder
}

func (s *allCapabilitiesStore) Add(ns, key string, value []byte, ttl time.Duration) (bool, error) {
	return s.adder.Add(ns, key, value, ttl)
}

func TestBulkAndAuditCapabilitiesPreserved(t *testing.T) {
	mem := newMem()
	backend := &bulkAuditStore{
		Store:       mem,
		foreignKeys: []string{"foreign/raw"},
	}
	s := securekv.New(backend, newBox(t), []string{allowNS})

	bulk, ok := s.(kv.BulkStore)
	if !ok {
		t.Fatal("wrapper must expose kv.BulkStore when the backend does")
	}
	auditor, ok := s.(kv.KeyAuditor)
	if !ok {
		t.Fatal("wrapper must expose kv.KeyAuditor when the backend does")
	}
	if _, ok := s.(kv.Adder); ok {
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
	if raw := mem.raw(allowNS, "42"); !secretbox.IsSealed(raw) || bytes.Contains(raw, []byte("octocat")) {
		t.Fatal("BulkSet did not encrypt the sensitive value at rest")
	}
	if got, err := s.Get(allowNS, "42"); err != nil || string(got) != "octocat" {
		t.Fatalf("Get after BulkSet = (%q, %v)", got, err)
	}

	if err := bulk.BulkDelete(allowNS, []string{"42"}); err != nil {
		t.Fatalf("BulkDelete: %v", err)
	}
	if backend.bulkDeletes != 1 {
		t.Fatalf("backend BulkDelete calls = %d, want 1", backend.bulkDeletes)
	}
	if keys, err := auditor.ForeignKeys(); err != nil || len(keys) != 1 || keys[0] != "foreign/raw" {
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
