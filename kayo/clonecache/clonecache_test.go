package clonecache

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/Hayao0819/Kamisato/internal/gitcmd"
)

// commitRepo writes body to PKGBUILD in dir, commits it, and returns the new HEAD.
func commitRepo(t *testing.T, dir, body string, first bool) string {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "PKGBUILD"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	steps := [][]string{{"add", "-A"}, {"commit", "--quiet", "-m", "c"}}
	if first {
		steps = append([][]string{
			{"init", "--quiet"}, {"config", "user.email", "t@t"}, {"config", "user.name", "t"},
		}, steps...)
	}
	for _, args := range steps {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v (%s)", args, err, out)
		}
	}
	head, err := gitcmd.HeadCommit(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	return head
}

func TestCheck(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	cache := Dir(root, "x")
	if err := os.MkdirAll(cache, 0o755); err != nil {
		t.Fatal(err)
	}
	c1 := commitRepo(t, cache, "pkgname=x\n", true)

	// Checkout at the approved commit is not drift.
	res, err := Check(ctx, root, "x", c1)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if !res.Exists || !res.Matches || res.Drifted() {
		t.Errorf("matching commit: %+v, want Exists && Matches && !Drifted", res)
	}

	// Advancing the checkout past the approved commit is drift.
	c2 := commitRepo(t, cache, "pkgname=x\n# tampered\n", false)
	if c2 == c1 {
		t.Fatal("second commit did not advance HEAD")
	}
	res, err = Check(ctx, root, "x", c1)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if !res.Drifted() || res.Head != c2 {
		t.Errorf("drifted checkout: %+v, want Drifted with Head=%s", res, c2)
	}

	// A pkgbase the helper has not cloned is not drift (nothing to build from).
	res, err = Check(ctx, root, "never-cloned", c1)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if res.Exists || res.Drifted() {
		t.Errorf("un-cloned pkgbase: %+v, want !Exists && !Drifted", res)
	}
}
