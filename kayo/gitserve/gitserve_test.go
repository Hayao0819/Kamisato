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
	c, err := gitcmd.Output(context.Background(), dir, "rev-parse", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	return dir, c[:len(c)-1] // strip newline
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
	head, err := gitcmd.Output(ctx, repo, "rev-parse", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if head[:len(head)-1] != commit {
		t.Errorf("served HEAD = %q, want pinned %q", head, commit)
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
