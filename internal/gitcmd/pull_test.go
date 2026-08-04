package gitcmd

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	git "github.com/go-git/go-git/v6"
)

func runTestGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// TestPull fast-forwards a local clone from a local origin — no network — and
// confirms a second pull with nothing to fetch is a no-op, not an error.
func TestPull(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	origin := t.TempDir()
	runTestGit(t, origin, "init", "-b", "main")
	if err := os.WriteFile(filepath.Join(origin, "a.txt"), []byte("one\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runTestGit(t, origin, "add", "a.txt")
	runTestGit(t, origin, "commit", "-m", "one")

	work := filepath.Join(t.TempDir(), "clone")
	if _, err := git.PlainClone(work, &git.CloneOptions{URL: origin}); err != nil {
		t.Fatalf("setup clone: %v", err)
	}

	// Advance origin, then pull it into the clone.
	if err := os.WriteFile(filepath.Join(origin, "b.txt"), []byte("two\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runTestGit(t, origin, "add", "b.txt")
	runTestGit(t, origin, "commit", "-m", "two")

	if err := Pull(context.Background(), work); err != nil {
		t.Fatalf("Pull: %v", err)
	}
	if _, err := os.Stat(filepath.Join(work, "b.txt")); err != nil {
		t.Errorf("pull did not fast-forward: b.txt missing: %v", err)
	}

	if err := Pull(context.Background(), work); err != nil {
		t.Errorf("no-op Pull (already up to date) should not error: %v", err)
	}
}

// TestIsCleanIgnoresUntracked pins the pull gate's dirtiness definition: an
// untracked file alone is clean (SyncHard keeps it), a tracked modification is
// not.
func TestIsCleanIgnoresUntracked(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	dir := t.TempDir()
	runTestGit(t, dir, "init", "-b", "main")
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("one\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runTestGit(t, dir, "add", "a.txt")
	runTestGit(t, dir, "commit", "-m", "one")

	if err := os.WriteFile(filepath.Join(dir, "note.md"), []byte("memo\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	clean, err := IsClean(dir)
	if err != nil {
		t.Fatalf("IsClean: %v", err)
	}
	if !clean {
		t.Error("untracked file alone should be clean")
	}

	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("changed\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	clean, err = IsClean(dir)
	if err != nil {
		t.Fatalf("IsClean: %v", err)
	}
	if clean {
		t.Error("tracked modification should not be clean")
	}
}

// TestSyncHardKeepsUntracked proves the reset SyncHard performs does not delete
// untracked files, which is what lets IsClean ignore them.
func TestSyncHardKeepsUntracked(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	origin := t.TempDir()
	runTestGit(t, origin, "init", "-b", "main")
	if err := os.WriteFile(filepath.Join(origin, "a.txt"), []byte("one\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runTestGit(t, origin, "add", "a.txt")
	runTestGit(t, origin, "commit", "-m", "one")

	work := filepath.Join(t.TempDir(), "clone")
	if _, err := git.PlainClone(work, &git.CloneOptions{URL: origin}); err != nil {
		t.Fatalf("setup clone: %v", err)
	}
	if err := os.WriteFile(filepath.Join(work, "note.md"), []byte("memo\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(origin, "a.txt"), []byte("two\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runTestGit(t, origin, "add", "a.txt")
	runTestGit(t, origin, "commit", "-m", "two")

	if err := SyncHard(context.Background(), work, "main"); err != nil {
		t.Fatalf("SyncHard: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(work, "a.txt"))
	if err != nil || string(got) != "two\n" {
		t.Errorf("worktree not synced to origin: %q, %v", got, err)
	}
	if _, err := os.Stat(filepath.Join(work, "note.md")); err != nil {
		t.Errorf("untracked file must survive SyncHard: %v", err)
	}
}
