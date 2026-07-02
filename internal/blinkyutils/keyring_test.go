package blinkyutils

import (
	"errors"
	"testing"
)

// fakeKeyring is an in-memory Keyring whose availability can be toggled to model a
// headless box with no Secret Service.
type fakeKeyring struct {
	data        map[string]string
	unavailable bool
}

var errUnavailable = errors.New("keyring unavailable")

func newFakeKeyring() *fakeKeyring { return &fakeKeyring{data: map[string]string{}} }

func (f *fakeKeyring) Get(_, key string) (string, error) {
	if f.unavailable {
		return "", errUnavailable
	}
	v, ok := f.data[key]
	if !ok {
		return "", errKeyringNotFound
	}
	return v, nil
}

func (f *fakeKeyring) Set(_, key, secret string) error {
	if f.unavailable {
		return errUnavailable
	}
	f.data[key] = secret
	return nil
}

func (f *fakeKeyring) Delete(_, key string) error {
	if f.unavailable {
		return errUnavailable
	}
	if _, ok := f.data[key]; !ok {
		return errKeyringNotFound
	}
	delete(f.data, key)
	return nil
}

// useKeyring swaps in a fake for the duration of a test.
func useKeyring(t *testing.T, k Keyring) {
	t.Helper()
	prev := secretKeyring
	secretKeyring = k
	t.Cleanup(func() { secretKeyring = prev })
}

// With a working keyring, StoreSecret accepts the secret (so the caller can clear
// the file-DB password), and LoadSecret reads it back from the keyring, never the
// file. This is the primary path: the token is not on disk in plaintext.
func TestStoreAndLoadUsesKeyring(t *testing.T) {
	fake := newFakeKeyring()
	useKeyring(t, fake)

	if stored := StoreSecret("repo.example.com", "s3cr3t"); !stored {
		t.Fatal("StoreSecret should report the keyring accepted the secret")
	}
	if got := fake.data["repo.example.com"]; got != "s3cr3t" {
		t.Fatalf("keyring holds %q, want the secret", got)
	}
	// fileSecret is empty because the caller cleared it after a keyring store.
	if got := LoadSecret("repo.example.com", ""); got != "s3cr3t" {
		t.Fatalf("LoadSecret = %q, want the keyring secret", got)
	}
}

// On a headless box (no Secret Service) StoreSecret reports failure so the caller
// keeps the secret in the file DB, and LoadSecret falls back to that file value.
func TestFallsBackToFileWhenKeyringUnavailable(t *testing.T) {
	fake := newFakeKeyring()
	fake.unavailable = true
	useKeyring(t, fake)

	if stored := StoreSecret("repo.example.com", "s3cr3t"); stored {
		t.Fatal("StoreSecret must report failure when the keyring is unavailable")
	}
	if got := LoadSecret("repo.example.com", "s3cr3t"); got != "s3cr3t" {
		t.Fatalf("LoadSecret = %q, want the file-DB fallback secret", got)
	}
}

// A secret that only exists in the file DB (a pre-keyring login) is still returned
// even when the keyring is up but has no entry for it — the migration read path.
func TestLoadMigratesFromFileOnKeyringMiss(t *testing.T) {
	fake := newFakeKeyring()
	useKeyring(t, fake)

	if got := LoadSecret("legacy.example.com", "old-token"); got != "old-token" {
		t.Fatalf("LoadSecret = %q, want the file-DB secret on a keyring miss", got)
	}
}

// When both stores hold a value the keyring wins, so a rotated token in the keyring
// is not shadowed by a stale file-DB entry.
func TestKeyringTakesPrecedenceOverFile(t *testing.T) {
	fake := newFakeKeyring()
	useKeyring(t, fake)
	_ = fake.Set(keyringService, "repo.example.com", "new-token")

	if got := LoadSecret("repo.example.com", "stale-file-token"); got != "new-token" {
		t.Fatalf("LoadSecret = %q, want the keyring secret to win", got)
	}
}

// ForgetSecret drops the keyring entry and tolerates a missing one.
func TestForgetSecret(t *testing.T) {
	fake := newFakeKeyring()
	useKeyring(t, fake)
	_ = fake.Set(keyringService, "repo.example.com", "s3cr3t")

	ForgetSecret("repo.example.com")
	if _, ok := fake.data["repo.example.com"]; ok {
		t.Fatal("ForgetSecret should remove the keyring entry")
	}
	ForgetSecret("repo.example.com") // second call is a no-op, not a panic
}
