package keycmd

import (
	"bytes"
	"strings"
	"testing"
)

// runKey executes the key command group with args and returns combined output.
func runKey(t *testing.T, args ...string) (string, error) {
	t.Helper()
	cmd := Cmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return buf.String(), err
}

func TestGenerateListExport(t *testing.T) {
	home := t.TempDir()

	out, err := runKey(t, "generate", "--key-home", home, "--name", "MyRepo", "--email", "repo@example.com")
	if err != nil {
		t.Fatalf("generate: %v\n%s", err, out)
	}
	if !strings.Contains(out, "Primary fingerprint:") {
		t.Errorf("generate output missing fingerprint:\n%s", out)
	}

	// Generating again must refuse rather than clobber the key.
	if _, err := runKey(t, "generate", "--key-home", home, "--name", "X", "--email", "x@x"); err == nil {
		t.Error("second generate should fail (key exists)")
	}

	listOut, err := runKey(t, "list", "--key-home", home)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if !strings.Contains(listOut, "primary") || !strings.Contains(listOut, "subkey") {
		t.Errorf("list should show primary and subkey:\n%s", listOut)
	}

	pubOut, err := runKey(t, "export", "--key-home", home)
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	if !strings.Contains(pubOut, "BEGIN PGP PUBLIC KEY BLOCK") {
		t.Errorf("export should emit an armored public key:\n%s", pubOut)
	}
	if strings.Contains(pubOut, "PRIVATE KEY") {
		t.Error("public export leaked private key material")
	}
}

func TestRotateKeepsPrimaryFingerprintViaCLI(t *testing.T) {
	home := t.TempDir()
	if _, err := runKey(t, "generate", "--key-home", home, "--name", "R", "--email", "r@example.com"); err != nil {
		t.Fatal(err)
	}
	before := fingerprintFromList(t, home)

	if _, err := runKey(t, "subkey", "rotate", "--key-home", home); err != nil {
		t.Fatalf("rotate: %v", err)
	}
	after := fingerprintFromList(t, home)
	if before != after {
		t.Errorf("primary fingerprint changed across rotate: %s -> %s", before, after)
	}
}

func fingerprintFromList(t *testing.T, home string) string {
	t.Helper()
	out, err := runKey(t, "list", "--key-home", home, "--json")
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, `"kind":"primary"`) {
			return line
		}
	}
	t.Fatalf("no primary row in list output:\n%s", out)
	return ""
}

func TestRevokeNeedsConfirmation(t *testing.T) {
	home := t.TempDir()
	if _, err := runKey(t, "generate", "--key-home", home, "--name", "R", "--email", "r@example.com"); err != nil {
		t.Fatal(err)
	}
	if _, err := runKey(t, "revoke", "--key-home", home, "--reason", "compromised"); err == nil {
		t.Error("revoke without --yes should fail")
	}
	if _, err := runKey(t, "revoke", "--key-home", home, "--reason", "bogus", "--yes"); err == nil {
		t.Error("revoke with an invalid reason should fail")
	}
}
