package plan

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"

	pkg "github.com/Hayao0819/Kamisato/internal/pacman/pkg"
	"github.com/Hayao0819/Kamisato/internal/pacman/repo"
	"github.com/Hayao0819/Kamisato/internal/pacman/source"
)

func vcsSrc(t *testing.T, base, version, sourceValue string, depends ...string) *pkg.SourcePackage {
	t.Helper()
	body := "pkgbase = " + base + "\n\tpkgver = " + version + "\n\tpkgrel = 1\n\tarch = x86_64\n"
	if sourceValue != "" {
		body += "\tsource = " + sourceValue + "\n"
	}
	for _, dependency := range depends {
		body += "\tmakedepends = " + dependency + "\n"
	}
	body += "\npkgname = " + base + "\n"
	return srcinfoPkg(t, body)
}

func TestGitCommitFromVersion(t *testing.T) {
	tests := []struct {
		version string
		commit  string
		ok      bool
	}{
		{"0.17.0.r1661.g637504176bd3-1", "637504176bd3", true},
		{"1:r10.gABCDEF1-2", "abcdef1", true},
		{"r1142.a17a017-1", "a17a017", true},
		{"1.2.3-1", "", false},
		{"1.0.g12345-1", "", false},
	}
	for _, tt := range tests {
		got, ok := gitCommitFromVersion(tt.version)
		if got != tt.commit || ok != tt.ok {
			t.Errorf("gitCommitFromVersion(%q) = %q, %t; want %q, %t", tt.version, got, ok, tt.commit, tt.ok)
		}
	}
}

func TestGitRemoteCommit(t *testing.T) {
	dir := t.TempDir()
	script := `#!/bin/sh
test "$GIT_TERMINAL_PROMPT" = 0 || exit 10
test "$1" = ls-remote || exit 11
test "$2" = -- || exit 12
test "$3" = https://example.com/project.git || exit 13
test "$4" = next || exit 14
printf 'abcdef1234567890abcdef1234567890abcdef12\trefs/heads/next\n'
`
	if err := os.WriteFile(filepath.Join(dir, "git"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	got, err := gitRemoteCommit(context.Background(), source.GitSource{
		Remote: "https://example.com/project.git",
		Ref:    "next",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got != "abcdef1234567890abcdef1234567890abcdef12" {
		t.Errorf("commit = %q", got)
	}
}

func TestDetectVCSUpdates(t *testing.T) {
	packages := []*pkg.SourcePackage{
		vcsSrc(t, "unchanged", "1", "git+https://example.com/unchanged.git"),
		vcsSrc(t, "changed", "1", "git+https://example.com/changed.git#branch=next"),
		vcsSrc(t, "untracked-version", "1", "git+https://example.com/untracked.git"),
		vcsSrc(t, "unreachable", "1", "git+https://example.com/unreachable.git"),
		vcsSrc(t, "pinned", "1", "git+https://example.com/pinned.git#tag=v1"),
		vcsSrc(t, "archive", "1", "https://example.com/source.tar.zst"),
	}
	remote := &repo.RemoteRepo{Pkgs: []*pkg.BinaryPackage{
		remoteBin("unchanged", "0.r1.g1111111-1"),
		remoteBin("changed", "0.r1.g2222222-1"),
		remoteBin("untracked-version", "1.0-1"),
		remoteBin("unreachable", "0.r1.g4444444-1"),
		remoteBin("pinned", "1-1"),
		remoteBin("archive", "1-1"),
	}}

	var mutex sync.Mutex
	called := map[string]int{}
	resolve := func(_ context.Context, gitSource source.GitSource) (string, error) {
		mutex.Lock()
		called[gitSource.Remote]++
		mutex.Unlock()
		switch gitSource.Remote {
		case "https://example.com/unchanged.git":
			return "1111111111111111111111111111111111111111", nil
		case "https://example.com/changed.git":
			return "3333333333333333333333333333333333333333", nil
		case "https://example.com/unreachable.git":
			return "", errors.New("unreachable")
		default:
			return "", errors.New("unexpected lookup")
		}
	}

	updates := detectVCSUpdates(context.Background(), packages, remote, "x86_64", resolve)
	if _, ok := updates["changed"]; !ok {
		t.Error("changed VCS source was not selected")
	}
	if _, ok := updates["untracked-version"]; !ok {
		t.Error("VCS package without a published commit was not selected")
	}
	if len(updates) != 2 {
		t.Errorf("updates = %v, want changed and untracked-version", updates)
	}
	if called["https://example.com/untracked.git"] != 0 || called["https://example.com/pinned.git"] != 0 {
		t.Errorf("unexpected lookups = %v", called)
	}
}

func TestVCSUpdateSeedsMakedependsCascade(t *testing.T) {
	upstream := vcsSrc(t, "upstream", "0.r1.g1111111", "git+https://example.com/upstream.git")
	dependent := vcsSrc(t, "dependent", "1", "", "upstream")
	remote := &repo.RemoteRepo{Pkgs: []*pkg.BinaryPackage{
		remoteBin("upstream", "0.r1.g1111111-1"),
		remoteBin("dependent", "1-1"),
	}}
	resolve := func(context.Context, source.GitSource) (string, error) {
		return "2222222222222222222222222222222222222222", nil
	}

	got, err := compute(context.Background(), []*pkg.SourcePackage{upstream, dependent}, remote, "x86_64", CascadeMakeDepends, 0, nil, resolve)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"upstream", "dependent"}; !reflect.DeepEqual(got.Order, want) {
		t.Errorf("order = %v, want %v", got.Order, want)
	}
	if got.Reasons["upstream"] != "vcs" || got.Reasons["dependent"] != "makedepends" {
		t.Errorf("reasons = %v", got.Reasons)
	}
	if want := []string{"dependent"}; !reflect.DeepEqual(got.BumpTargets, want) {
		t.Errorf("bump targets = %v, want %v", got.BumpTargets, want)
	}
}

func TestVCSLookupCancellationStopsPlan(t *testing.T) {
	packageSource := vcsSrc(t, "vcs", "0.r1.g1111111", "git+https://example.com/vcs.git")
	remote := &repo.RemoteRepo{Pkgs: []*pkg.BinaryPackage{remoteBin("vcs", "0.r1.g1111111-1")}}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	resolve := func(ctx context.Context, _ source.GitSource) (string, error) {
		return "", ctx.Err()
	}

	if _, err := compute(ctx, []*pkg.SourcePackage{packageSource}, remote, "x86_64", CascadeOff, 0, nil, resolve); !errors.Is(err, context.Canceled) {
		t.Fatalf("compute error = %v, want context cancellation", err)
	}
}
