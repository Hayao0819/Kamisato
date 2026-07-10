package service_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Hayao0819/Kamisato/internal/errors"

	"github.com/Hayao0819/Kamisato/ayato/domain"
	"github.com/Hayao0819/Kamisato/ayato/repository"
	"github.com/Hayao0819/Kamisato/ayato/repository/blob"
	"github.com/Hayao0819/Kamisato/ayato/service"
	"github.com/Hayao0819/Kamisato/internal/conf"
)

// newTieredService wires a real repository stack (localfs blob + BadgerDB kv) so
// tier promotion exercises the actual storage and CAS database commit, not a
// mock. It returns the service, the config, and the on-disk repo root (for
// counting stored package files).
func newTieredService(t *testing.T, repos []conf.BinRepoConfig) (*service.Service, *conf.AyatoConfig, string) {
	t.Helper()
	repoDir := t.TempDir()
	cfg := &conf.AyatoConfig{Repos: repos}
	cfg.Store.LocalRepoDir = repoDir
	cfg.Store.BadgerDB = t.TempDir()

	name, bin, _, kvStore, err := repository.New(cfg)
	if err != nil {
		t.Fatalf("repository.New: %v", err)
	}
	t.Cleanup(func() { _ = kvStore.Close() })

	svc := service.New(name, bin, nil, nil, cfg)
	if err := svc.InitAll(); err != nil {
		t.Fatalf("InitAll: %v", err)
	}
	return svc, cfg, repoDir
}

func uploadPkg(t *testing.T, svc *service.Service, repo, pkgname string) {
	t.Helper()
	fname := pkgname + "-1.0-1-x86_64.pkg.tar.zst"
	files := &domain.UploadFiles{PkgFile: pkgStream(fname, buildNamedPkg(t, pkgname))}
	if err := svc.UploadFile(repo, files); err != nil {
		t.Fatalf("upload %s to %s: %v", pkgname, repo, err)
	}
}

func pkgNames(t *testing.T, svc *service.Service, repo, arch string) []string {
	t.Helper()
	pkgs, err := svc.Pkgs(repo, arch)
	if err != nil {
		// A tier with no published package has no db yet; that is an empty tier.
		if errors.Is(err, blob.ErrNotFound) {
			return nil
		}
		t.Fatalf("Pkgs(%s, %s): %v", repo, arch, err)
	}
	names := make([]string, 0, len(pkgs.Packages))
	for _, p := range pkgs.Packages {
		names = append(names, p.PkgName)
	}
	return names
}

// pkgFileCount counts the package files stored on disk across every tier, so a
// test can prove a moved package lives in exactly one tier at a time (promotion
// copies to the target tier, then the move policy drops the source).
func pkgFileCount(t *testing.T, repoDir string) int {
	t.Helper()
	n := 0
	err := filepath.WalkDir(repoDir, func(_ string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.Contains(d.Name(), ".pkg.tar.") {
			n++
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk repo dir: %v", err)
	}
	return n
}

func has(names []string, want string) bool {
	for _, n := range names {
		if n == want {
			return true
		}
	}
	return false
}

// TestTieredPromotionFlow walks a package staging -> testing -> stable and checks
// each step: an upload lands in staging only, a promotion makes the package
// appear in the next tier's database, exactly one package file exists across all
// tiers (the move policy drops the source after each promotion), and the move
// policy clears the source tier.
func TestTieredPromotionFlow(t *testing.T) {
	svc, _, repoDir := newTieredService(t, []conf.BinRepoConfig{
		{Name: "myrepo", Tiered: true},
	})
	ctx := context.Background()

	// An upload addressed at the logical repo lands in staging only.
	uploadPkg(t, svc, "myrepo", "foo")
	if !has(pkgNames(t, svc, "myrepo-staging", "x86_64"), "foo") {
		t.Fatal("upload did not land in the staging tier")
	}
	if len(pkgNames(t, svc, "myrepo-testing", "x86_64")) != 0 {
		t.Fatal("upload leaked into the testing tier")
	}
	if len(pkgNames(t, svc, "myrepo", "x86_64")) != 0 {
		t.Fatal("upload leaked into the stable tier")
	}
	if n := pkgFileCount(t, repoDir); n != 1 {
		t.Fatalf("package files after upload = %d, want 1", n)
	}

	// staging -> testing: appears in testing, gone from staging (move policy), and
	// exactly one package file remains on disk (moved, not duplicated).
	if err := svc.PromotePackage(ctx, "myrepo", conf.TierStaging, conf.TierTesting, "foo", "1.0-1"); err != nil {
		t.Fatalf("promote staging->testing: %v", err)
	}
	if !has(pkgNames(t, svc, "myrepo-testing", "x86_64"), "foo") {
		t.Fatal("promotion did not add foo to the testing tier")
	}
	if len(pkgNames(t, svc, "myrepo-staging", "x86_64")) != 0 {
		t.Fatal("move policy did not clear foo from the staging tier")
	}
	if n := pkgFileCount(t, repoDir); n != 1 {
		t.Fatalf("package files after promotion = %d, want 1 (moved, not duplicated)", n)
	}

	// testing -> stable: appears in stable, gone from testing.
	if err := svc.PromotePackage(ctx, "myrepo", conf.TierTesting, conf.TierStable, "foo", ""); err != nil {
		t.Fatalf("promote testing->stable: %v", err)
	}
	if !has(pkgNames(t, svc, "myrepo", "x86_64"), "foo") {
		t.Fatal("promotion did not add foo to the stable tier")
	}
	if len(pkgNames(t, svc, "myrepo-testing", "x86_64")) != 0 {
		t.Fatal("move policy did not clear foo from the testing tier")
	}
	if n := pkgFileCount(t, repoDir); n != 1 {
		t.Fatalf("package files after full promotion = %d, want 1", n)
	}
}

// TestTieredPromotionKeepInSource proves the keep-in-source policy leaves the
// package published in both tiers after a promotion.
func TestTieredPromotionKeepInSource(t *testing.T) {
	svc, _, _ := newTieredService(t, []conf.BinRepoConfig{
		{Name: "myrepo", Tiered: true, PromotionKeepInSource: true},
	})

	uploadPkg(t, svc, "myrepo", "foo")
	if err := svc.PromotePackage(context.Background(), "myrepo", conf.TierStaging, conf.TierTesting, "foo", ""); err != nil {
		t.Fatalf("promote: %v", err)
	}
	if !has(pkgNames(t, svc, "myrepo-testing", "x86_64"), "foo") {
		t.Fatal("promotion did not add foo to the testing tier")
	}
	if !has(pkgNames(t, svc, "myrepo-staging", "x86_64"), "foo") {
		t.Fatal("keep-in-source policy dropped foo from the staging tier")
	}
}

// TestPromotionRejectsInvalidRequests locks the guards: a non-tiered repo, a
// non-adjacent tier step, a version mismatch, and an absent package are all
// refused, and a refused promotion leaves the target tier untouched (the CAS
// commit is all-or-nothing — a target tier db is never half-updated).
func TestPromotionRejectsInvalidRequests(t *testing.T) {
	svc, _, _ := newTieredService(t, []conf.BinRepoConfig{
		{Name: "myrepo", Tiered: true},
		{Name: "single"},
	})
	ctx := context.Background()
	uploadPkg(t, svc, "myrepo", "foo")

	// Non-tiered repo.
	if err := svc.PromotePackage(ctx, "single", conf.TierStaging, conf.TierTesting, "foo", ""); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("promote on non-tiered repo = %v, want ErrInvalid", err)
	}
	// Non-adjacent step (staging -> stable).
	if err := svc.PromotePackage(ctx, "myrepo", conf.TierStaging, conf.TierStable, "foo", ""); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("skip-tier promote = %v, want ErrInvalid", err)
	}
	// Version mismatch.
	if err := svc.PromotePackage(ctx, "myrepo", conf.TierStaging, conf.TierTesting, "foo", "9.9-9"); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("version-mismatch promote = %v, want ErrInvalid", err)
	}
	// Absent package.
	if err := svc.PromotePackage(ctx, "myrepo", conf.TierStaging, conf.TierTesting, "ghost", ""); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("absent-package promote = %v, want ErrNotFound", err)
	}
	// Every refused promotion left the testing tier empty.
	if len(pkgNames(t, svc, "myrepo-testing", "x86_64")) != 0 {
		t.Fatal("a refused promotion half-updated the testing tier")
	}
}

// TestTieredOffUnchanged proves a repo without tiering behaves exactly as before:
// an upload lands directly in the single repo and there are no tier repos.
func TestTieredOffUnchanged(t *testing.T) {
	svc, cfg, _ := newTieredService(t, []conf.BinRepoConfig{
		{Name: "single"},
	})
	if names := cfg.PhysicalRepoNames(); len(names) != 1 || names[0] != "single" {
		t.Fatalf("PhysicalRepoNames = %v, want [single] for a non-tiered repo", names)
	}
	uploadPkg(t, svc, "single", "foo")
	if !has(pkgNames(t, svc, "single", "x86_64"), "foo") {
		t.Fatal("upload to a non-tiered repo did not land in the repo")
	}
}
