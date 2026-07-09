package secretbox

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"filippo.io/age"
)

func newBox(t *testing.T) SecretBox {
	t.Helper()
	id, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatalf("generate identity: %v", err)
	}
	box, err := NewAgeX25519(id.String())
	if err != nil {
		t.Fatalf("NewAgeX25519: %v", err)
	}
	return box
}

func TestSealOpenRoundTrip(t *testing.T) {
	box := newBox(t)
	plain := []byte("super-secret allowlist entry")

	sealed, err := box.Seal(plain)
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}
	opened, err := box.Open(sealed)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if !bytes.Equal(opened, plain) {
		t.Fatalf("round-trip = %q, want %q", opened, plain)
	}
}

// A sealed value must not leak the plaintext and must be recognizable as an age
// ciphertext (so the migration path can tell sealed from plaintext).
func TestSealedIsNotPlaintext(t *testing.T) {
	box := newBox(t)
	plain := []byte("hunter2")

	sealed, err := box.Seal(plain)
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}
	if bytes.Contains(sealed, plain) {
		t.Fatal("sealed value must not contain the plaintext")
	}
	if !IsSealed(sealed) {
		t.Fatal("sealed value must be detected as an age ciphertext")
	}
	if IsSealed(plain) {
		t.Fatal("a plaintext value must not be mistaken for a ciphertext")
	}
}

// A ciphertext sealed to one key must not open with an unrelated identity.
func TestOpenWithWrongKeyFails(t *testing.T) {
	sealed, err := newBox(t).Seal([]byte("secret"))
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}
	if _, err := newBox(t).Open(sealed); err == nil {
		t.Fatal("opening with an unrelated identity must fail")
	}
}

func TestLoadAgeIdentity(t *testing.T) {
	id, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	// A literal value wins over the file.
	got, err := LoadAgeIdentity(id.String(), "/does/not/matter")
	if err != nil || got != id.String() {
		t.Fatalf("literal value: got %q err %v", got, err)
	}

	// A file with comments and blank lines yields the first key line.
	dir := t.TempDir()
	path := filepath.Join(dir, "key.txt")
	content := "# created by age\n\n" + id.String() + "\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err = LoadAgeIdentity("", path)
	if err != nil || got != id.String() {
		t.Fatalf("from file: got %q err %v", got, err)
	}

	// Neither configured leaves encryption off (empty, no error).
	got, err = LoadAgeIdentity("", "")
	if err != nil || got != "" {
		t.Fatalf("unset: got %q err %v", got, err)
	}
}
