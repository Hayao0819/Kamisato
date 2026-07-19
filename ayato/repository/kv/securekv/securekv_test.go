package securekv_test

import (
	"bytes"
	"testing"
	"time"

	"github.com/Hayao0819/Kamisato/ayato/repository/kv"
	"github.com/Hayao0819/Kamisato/ayato/repository/kv/securekv"
	"github.com/Hayao0819/Kamisato/internal/auth/secretbox"
)

func TestEncryptedNamespaceRoundTrip(t *testing.T) {
	mem := newMem()
	store := securekv.New(mem, newBox(t), []string{allowNS})

	if err := store.Set(allowNS, "42", []byte("octocat"), 0); err != nil {
		t.Fatalf("Set: %v", err)
	}
	stored := mem.raw(allowNS, "42")
	if bytes.Contains(stored, []byte("octocat")) {
		t.Fatal("value must be encrypted at rest")
	}
	if !secretbox.IsSealed(stored) {
		t.Fatal("stored value must be an age ciphertext")
	}

	got, err := store.Get(allowNS, "42")
	if err != nil || string(got) != "octocat" {
		t.Fatalf("Get = %q, %v; want octocat", got, err)
	}
	entries, err := store.List(allowNS)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(entries) != 1 ||
		entries[0].Key != "42" ||
		string(entries[0].Value) != "octocat" {
		t.Fatalf("List = %+v, want one decrypted entry", entries)
	}
}

func TestReadsPreEncryptionPlaintext(t *testing.T) {
	mem := newMem()
	if err := mem.Set(allowNS, "7", []byte("legacy"), 0); err != nil {
		t.Fatal(err)
	}

	store := securekv.New(mem, newBox(t), []string{allowNS})
	got, err := store.Get(allowNS, "7")
	if err != nil || string(got) != "legacy" {
		t.Fatalf("Get = %q, %v; want legacy", got, err)
	}
	entries, err := store.List(allowNS)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(entries) != 1 || string(entries[0].Value) != "legacy" {
		t.Fatalf("List = %+v, want legacy plaintext", entries)
	}
}

func TestUnencryptedNamespaceIsPlaintext(t *testing.T) {
	mem := newMem()
	store := securekv.New(mem, newBox(t), []string{allowNS})

	if err := store.Set("replay", "nonce", []byte("1"), 0); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if got := mem.raw("replay", "nonce"); string(got) != "1" {
		t.Fatalf("unencrypted namespace stored %q, want plaintext 1", got)
	}
}

func TestDisabledIsPassthrough(t *testing.T) {
	mem := newMem()
	if store := securekv.New(mem, nil, []string{allowNS}); store != kv.Store(mem) {
		t.Fatal("a nil box must return the inner store unchanged")
	}
}

func TestAdderPreservedAndSeals(t *testing.T) {
	mem := newMem()
	store := securekv.New(mem, newBox(t), []string{allowNS})
	adder, ok := store.(kv.Adder)
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
	got, err := store.Get(allowNS, "9")
	if err != nil || string(got) != "mona" {
		t.Fatalf("Get after encrypted Add = (%q, %v)", got, err)
	}
}
