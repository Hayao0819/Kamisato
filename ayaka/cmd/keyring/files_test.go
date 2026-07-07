package keyringcmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFilesWritesThreeFiles(t *testing.T) {
	home := newKeyHome(t)
	out := t.TempDir()

	stdout, err := runKeyring(t, "files",
		"--key-home", home,
		"--name", "alterlinux",
		"--output-dir", out,
	)
	if err != nil {
		t.Fatalf("files: %v\n%s", err, stdout)
	}

	for _, name := range []string{"alterlinux.gpg", "alterlinux-trusted", "alterlinux-revoked"} {
		p := filepath.Join(out, name)
		if _, err := os.Stat(p); err != nil {
			t.Errorf("missing %s: %v", name, err)
		}
	}
	// -trusted lists the primary fingerprint with :4:.
	trusted, err := os.ReadFile(filepath.Join(out, "alterlinux-trusted"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(strings.TrimSpace(string(trusted)), ":4:") {
		t.Errorf("unexpected -trusted content: %q", string(trusted))
	}
}

func TestFilesRequiresNameAndDir(t *testing.T) {
	home := newKeyHome(t)
	if _, err := runKeyring(t, "files", "--key-home", home, "--output-dir", t.TempDir()); err == nil {
		t.Error("files without --name should fail")
	}
	if _, err := runKeyring(t, "files", "--key-home", home, "--name", "x"); err == nil {
		t.Error("files without --output-dir should fail")
	}
}
