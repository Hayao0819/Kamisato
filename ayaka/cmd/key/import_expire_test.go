package keycmd

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/Hayao0819/Kamisato/pkg/pacman/sign"
)

func TestImportFromStdin(t *testing.T) {
	// Produce an armored secret key to import.
	src := t.TempDir()
	orig, err := sign.GenerateSigningKey(src, "R", "r@example.com", 0, 365*24*time.Hour, "")
	if err != nil {
		t.Fatal(err)
	}
	armored, err := orig.ExportSecretArmored("")
	if err != nil {
		t.Fatal(err)
	}

	home := t.TempDir()
	cmd := Cmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetIn(strings.NewReader(armored))
	cmd.SetArgs([]string{"import", "-", "--key-home", home})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("import: %v\n%s", err, out.String())
	}
	if !strings.Contains(out.String(), orig.PrimaryFingerprint()) {
		t.Errorf("import should report the fingerprint:\n%s", out.String())
	}

	// The imported key is now managed: list shows it.
	listOut, err := runKey(t, "list", "--key-home", home)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(listOut, orig.PrimaryFingerprint()) {
		t.Errorf("imported key not listed:\n%s", listOut)
	}
}

func TestExpireExtendsViaCLI(t *testing.T) {
	home := t.TempDir()
	// A short-lived primary so extension is observable.
	if _, err := sign.GenerateSigningKey(home, "R", "r@example.com", time.Hour, 365*24*time.Hour, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := runKey(t, "expire", "--key-home", home, "--expire", "43800h", "--subkeys"); err != nil {
		t.Fatalf("expire: %v", err)
	}
	k, err := sign.LoadSigningKey(home, "")
	if err != nil {
		t.Fatal(err)
	}
	if k.PrimaryExpiry().Before(time.Now().Add(3 * 365 * 24 * time.Hour)) {
		t.Errorf("primary expiry not extended: %v", k.PrimaryExpiry())
	}

	// --expire is required.
	if _, err := runKey(t, "expire", "--key-home", home); err == nil {
		t.Error("expire without --expire should fail")
	}
}
