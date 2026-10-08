package gitserve

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/Hayao0819/Kamisato/internal/vcs/git"
)

func initRepo(t *testing.T) (dir, commit string) {
	t.Helper()
	dir = t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "PKGBUILD"), []byte("pkgname=x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"init", "--quiet"},
		{"config", "user.email", "t@t"},
		{"config", "user.name", "t"},
		{"add", "-A"},
		{"commit", "--quiet", "-m", "v1"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v (%s)", args, err, out)
		}
	}
	c, err := git.HeadCommit(context.Background(), dir)
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
	head, err := git.HeadCommit(ctx, repo)
	if err != nil {
		t.Fatal(err)
	}
	if head != commit {
		t.Errorf("served HEAD = %q, want pinned %q", head, commit)
	}
	if err := Materialize(ctx, root, "x", src, commit); err != nil {
		t.Fatalf("replace existing pin: %v", err)
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 1 || entries[0].Name() != "x.git" {
		t.Fatalf("pin staging should be cleaned: entries=%v, error=%v", entries, err)
	}
}

func TestMaterializeFailurePreservesExistingPin(t *testing.T) {
	src, commit := initRepo(t)
	for _, failure := range []string{"cancelled", "clone", "missing commit"} {
		t.Run(failure, func(t *testing.T) {
			root := t.TempDir()
			if err := Materialize(context.Background(), root, "x", src, commit); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			source, pin := src, commit
			switch failure {
			case "cancelled":
				cancel()
			case "clone":
				source = filepath.Join(t.TempDir(), "missing")
			case "missing commit":
				pin = "ffffffffffffffffffffffffffffffffffffffff"
			}
			err := Materialize(ctx, root, "x", source, pin)
			if err == nil {
				t.Fatal("expected the pin update to fail")
			}
			if failure == "cancelled" && !errors.Is(err, context.Canceled) {
				t.Fatalf("cancellation error=%v", err)
			}
			head, err := git.HeadCommit(context.Background(), filepath.Join(root, "x.git"))
			if err != nil || head != commit {
				t.Fatalf("previous pin lost: HEAD=%q, error=%v", head, err)
			}
			entries, err := os.ReadDir(root)
			if err != nil || len(entries) != 1 || entries[0].Name() != "x.git" {
				t.Fatalf("failed staging should be cleaned: entries=%v, error=%v", entries, err)
			}
		})
	}
}

func TestMaterializeDetachedRevisionSurvivesSourceCleanup(t *testing.T) {
	source, reviewedCommit := initRepo(t)
	if err := os.WriteFile(filepath.Join(source, "PKGBUILD"), []byte("pkgname=changed\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"add", "PKGBUILD"}, {"commit", "--quiet", "-m", "new HEAD"}} {
		command := exec.Command("git", args...)
		command.Dir = source
		if out, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v (%s)", args, err, out)
		}
	}
	ctx := context.Background()
	review, cleanup, err := git.CloneTemp(ctx, "kayo-review-test-*", git.CloneOptions{URL: source, Ref: reviewedCommit})
	defer cleanup()
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	if err := Materialize(ctx, root, "x", review, reviewedCommit); err != nil {
		t.Fatal(err)
	}
	cleanup()
	checkout, closeCheckout, err := git.CloneTemp(ctx, "kayo-served-test-*", git.CloneOptions{URL: filepath.Join(root, "x.git")})
	defer closeCheckout()
	if err != nil {
		t.Fatalf("pin must remain clonable without its temporary source: %v", err)
	}
	contents, err := os.ReadFile(filepath.Join(checkout, "PKGBUILD"))
	if err != nil || string(contents) != "pkgname=x\n" {
		t.Fatalf("served recipe=%q, error=%v; want the reviewed revision", contents, err)
	}
}

func TestRepositoryEffectsRejectUnsafePackageBases(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "served")
	victim := filepath.Join(parent, "victim.git")
	if err := os.Mkdir(victim, 0o700); err != nil {
		t.Fatal(err)
	}
	sentinel := filepath.Join(victim, "keep")
	if err := os.WriteFile(sentinel, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, base := range []string{"", ".", "..", "../victim", "nested/pkg", `nested\pkg`, "/absolute", "pkg\x00name"} {
		if err := Remove(root, base); err == nil {
			t.Errorf("Remove(%q) should refuse an unsafe package base", base)
		}
		if err := Materialize(context.Background(), root, base, "unused", "commit"); err == nil {
			t.Errorf("Materialize(%q) should refuse an unsafe package base", base)
		}
	}
	if _, err := os.Stat(root); !os.IsNotExist(err) {
		t.Errorf("invalid inputs created the served root: %v", err)
	}
	if contents, err := os.ReadFile(sentinel); err != nil || string(contents) != "keep" {
		t.Fatalf("repository outside the root was modified: contents=%q error=%v", contents, err)
	}
	if err := Remove("", "pkg"); err == nil {
		t.Error("Remove without a served root should fail")
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
