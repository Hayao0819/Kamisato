package gitcmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCommitPaths(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	dir := t.TempDir()
	runTestGit(t, dir, "init", "-b", "main")
	runTestGit(t, dir, "config", "user.name", "t")
	runTestGit(t, dir, "config", "user.email", "t@t")
	// The host's global commit.gpgsign would make go-git demand a signer plugin.
	runTestGit(t, dir, "config", "commit.gpgsign", "false")

	pkgdir := filepath.Join(dir, "pkg")
	if err := os.MkdirAll(pkgdir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"PKGBUILD", ".SRCINFO"} {
		if err := os.WriteFile(filepath.Join(pkgdir, f), []byte("pkgrel=1\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	hash, err := CommitPaths(dir, []string{"pkg/PKGBUILD", "pkg/.SRCINFO"}, "bump")
	if err != nil {
		t.Fatalf("CommitPaths: %v", err)
	}
	if len(hash) != 40 {
		t.Errorf("hash = %q, want a full sha", hash)
	}

	// Unrelated staged changes must not be swept into a later bump commit.
	if err := os.WriteFile(filepath.Join(dir, "other.txt"), []byte("x\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runTestGit(t, dir, "add", "other.txt")
	if err := os.WriteFile(filepath.Join(pkgdir, "PKGBUILD"), []byte("pkgrel=1.1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := CommitPaths(dir, []string{"pkg/PKGBUILD"}, "bump"); err == nil ||
		!strings.Contains(err.Error(), "staged") {
		t.Errorf("CommitPaths with unrelated staged changes = %v, want staged-changes error", err)
	}
}
