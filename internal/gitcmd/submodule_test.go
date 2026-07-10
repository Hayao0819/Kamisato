package gitcmd

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	git "github.com/go-git/go-git/v6"
)

func TestAddSubmodule(t *testing.T) {
	runGit := func(dir string, args ...string) {
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
	// origin repo that becomes the submodule
	origin := t.TempDir()
	runGit(origin, "init", "-q", "-b", "main")
	if err := os.WriteFile(filepath.Join(origin, "f.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(origin, "add", "-A")
	runGit(origin, "commit", "-q", "-m", "one")

	// superproject
	super := t.TempDir()
	runGit(super, "init", "-q", "-b", "main")
	if err := os.WriteFile(filepath.Join(super, "g.txt"), []byte("y"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(super, "add", "-A")
	runGit(super, "commit", "-q", "-m", "init")

	if err := AddSubmodule(context.Background(), super, origin, "sub"); err != nil {
		t.Fatalf("AddSubmodule: %v", err)
	}

	b, err := os.ReadFile(filepath.Join(super, ".gitmodules"))
	if err != nil {
		t.Fatalf("read .gitmodules: %v", err)
	}
	if !strings.Contains(string(b), "sub") {
		t.Errorf(".gitmodules missing submodule:\n%s", b)
	}
	if _, err := os.Stat(filepath.Join(super, "sub", "f.txt")); err != nil {
		t.Errorf("submodule not checked out: %v", err)
	}

	repo, err := git.PlainOpen(super)
	if err != nil {
		t.Fatal(err)
	}
	wt, err := repo.Worktree()
	if err != nil {
		t.Fatal(err)
	}
	subs, err := wt.Submodules()
	if err != nil {
		t.Fatal(err)
	}
	if len(subs) != 1 || subs[0].Config().Path != "sub" {
		t.Fatalf("submodules = %+v, want one at 'sub'", subs)
	}
}
