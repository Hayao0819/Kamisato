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

func newTieredService(
	t *testing.T,
	repos []conf.BinRepoConfig,
) (*service.Service, *conf.AyatoConfig, string) {
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

func uploadFoo(t *testing.T, svc *service.Service, repo string) {
	t.Helper()
	const pkgname = "foo"
	filename := pkgname + "-1.0-1-x86_64.pkg.tar.zst"
	files := &domain.UploadFiles{
		PkgFile: pkgStream(filename, buildPackage(t, pkgname, "1.0-1", "x86_64")),
	}
	if err := svc.UploadFile(repo, files); err != nil {
		t.Fatalf("upload %s to %s: %v", pkgname, repo, err)
	}
}

func pkgNames(t *testing.T, svc *service.Service, repo, arch string) []string {
	t.Helper()
	pkgs, err := svc.Pkgs(repo, arch)
	if err != nil {
		if errors.Is(err, blob.ErrNotFound) {
			return nil
		}
		t.Fatalf("Pkgs(%s, %s): %v", repo, arch, err)
	}
	names := make([]string, 0, len(pkgs.Packages))
	for _, pkg := range pkgs.Packages {
		names = append(names, pkg.PkgName)
	}
	return names
}

func pkgFileCount(t *testing.T, repoDir string) int {
	t.Helper()
	count := 0
	err := filepath.WalkDir(repoDir, func(_ string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() && strings.Contains(entry.Name(), ".pkg.tar.") {
			count++
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk repo dir: %v", err)
	}
	return count
}

func has(names []string, want string) bool {
	for _, name := range names {
		if name == want {
			return true
		}
	}
	return false
}

func TestTieredPromotionFlow(t *testing.T) {
	svc, _, repoDir := newTieredService(t, []conf.BinRepoConfig{
		{Name: "myrepo", Tiered: true},
	})
	ctx := context.Background()

	uploadFoo(t, svc, "myrepo")
	if !has(pkgNames(t, svc, "myrepo-staging", "x86_64"), "foo") {
		t.Fatal("upload did not land in the staging tier")
	}
	if len(pkgNames(t, svc, "myrepo-testing", "x86_64")) != 0 {
		t.Fatal("upload leaked into the testing tier")
	}
	if len(pkgNames(t, svc, "myrepo", "x86_64")) != 0 {
		t.Fatal("upload leaked into the stable tier")
	}
	if count := pkgFileCount(t, repoDir); count != 1 {
		t.Fatalf("package files after upload = %d, want 1", count)
	}

	err := svc.PromotePackage(
		ctx, "myrepo", domain.TierStaging, domain.TierTesting, "foo", "1.0-1",
	)
	if err != nil {
		t.Fatalf("promote staging->testing: %v", err)
	}
	if !has(pkgNames(t, svc, "myrepo-testing", "x86_64"), "foo") {
		t.Fatal("promotion did not add foo to the testing tier")
	}
	if len(pkgNames(t, svc, "myrepo-staging", "x86_64")) != 0 {
		t.Fatal("move policy did not clear foo from the staging tier")
	}
	if count := pkgFileCount(t, repoDir); count != 1 {
		t.Fatalf("package files after promotion = %d, want 1", count)
	}

	err = svc.PromotePackage(
		ctx, "myrepo", domain.TierTesting, domain.TierStable, "foo", "",
	)
	if err != nil {
		t.Fatalf("promote testing->stable: %v", err)
	}
	if !has(pkgNames(t, svc, "myrepo", "x86_64"), "foo") {
		t.Fatal("promotion did not add foo to the stable tier")
	}
	if len(pkgNames(t, svc, "myrepo-testing", "x86_64")) != 0 {
		t.Fatal("move policy did not clear foo from the testing tier")
	}
	if count := pkgFileCount(t, repoDir); count != 1 {
		t.Fatalf("package files after full promotion = %d, want 1", count)
	}
}
