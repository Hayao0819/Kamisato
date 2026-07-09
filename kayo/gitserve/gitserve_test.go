package gitserve

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/Hayao0819/Kamisato/internal/gitcmd"
)

func initRepo(t *testing.T) (dir, commit string) {
	t.Helper()
	dir = t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "PKGBUILD"), []byte("pkgname=x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"init", "--quiet"}, {"config", "user.email", "t@t"}, {"config", "user.name", "t"},
		{"add", "-A"}, {"commit", "--quiet", "-m", "v1"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v (%s)", args, err, out)
		}
	}
	c, err := gitcmd.HeadCommit(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	return dir, c
}

func TestMaterialize(t *testing.T) {
	src, commit := initRepo(t)
	root := t.TempDir()
	ctx := context.Background()

	if err := Materialize(ctx, root, "x", src, commit); err != nil {
		t.Fatalf("Materialize: %v", err)
	}
	repo := filepath.Join(root, "x.git")
	if _, err := os.Stat(filepath.Join(repo, "info", "refs")); err != nil {
		t.Errorf("dumb-HTTP info/refs not generated: %v", err)
	}
	head, err := gitcmd.HeadCommit(ctx, repo)
	if err != nil {
		t.Fatal(err)
	}
	if head != commit {
		t.Errorf("served HEAD = %q, want pinned %q", head, commit)
	}
}

func TestMaterializePins(t *testing.T) {
	src, commit := initRepo(t)
	root := t.TempDir()
	ctx := context.Background()

	// "approved" is pinned; "unapproved" has a checkout but no pin; "pinless" has an
	// approval with an empty commit. Only the approved one should be served.
	sources := map[string]string{"approved": src, "unapproved": src, "pinless": src}
	pin := func(pkgbase string) (string, bool) {
		switch pkgbase {
		case "approved":
			return commit, true
		case "pinless":
			return "", true
		default:
			return "", false
		}
	}

	n, err := MaterializePins(ctx, root, sources, pin)
	if err != nil {
		t.Fatalf("MaterializePins: %v", err)
	}
	if n != 1 {
		t.Fatalf("served %d pins, want 1", n)
	}
	if _, err := os.Stat(filepath.Join(root, "approved.git")); err != nil {
		t.Errorf("approved pin not materialized: %v", err)
	}
	for _, base := range []string{"unapproved", "pinless"} {
		if _, err := os.Stat(filepath.Join(root, base+".git")); !os.IsNotExist(err) {
			t.Errorf("%s should not be served (stat err=%v)", base, err)
		}
	}

	// The Handler serves the materialized pin and falls through for the rest.
	var fellThrough bool
	fallback := http.HandlerFunc(func(http.ResponseWriter, *http.Request) { fellThrough = true })
	h := NewHandler(root, fallback)
	for _, c := range []struct {
		path     string
		wantFall bool
	}{
		{"/approved.git/info/refs", false},
		{"/unapproved.git/info/refs", true},
	} {
		fellThrough = false
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, c.path, nil))
		if fellThrough != c.wantFall {
			t.Errorf("%s: fellThrough=%v, want %v", c.path, fellThrough, c.wantFall)
		}
	}
}

func TestHandlerRouting(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "managed.git"), 0o755); err != nil {
		t.Fatal(err)
	}

	var fellThrough bool
	fallback := http.HandlerFunc(func(http.ResponseWriter, *http.Request) { fellThrough = true })
	h := NewHandler(root, fallback)

	cases := []struct {
		path     string
		wantFall bool
	}{
		{"/managed.git/info/refs", false},  // served locally
		{"/unmanaged.git/info/refs", true}, // no local repo -> fallback (redirect)
		{"/rpc?v=5&type=search&arg=x", true},
	}
	for _, c := range cases {
		fellThrough = false
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, c.path, nil))
		if fellThrough != c.wantFall {
			t.Errorf("%s: fellThrough=%v, want %v", c.path, fellThrough, c.wantFall)
		}
	}
}
