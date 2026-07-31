package plan

import (
	"context"
	stderrors "errors"
	"reflect"
	"testing"

	pkg "github.com/Hayao0819/Kamisato/internal/pacman"
	"github.com/Hayao0819/Kamisato/internal/pacman/repo"
	"github.com/Hayao0819/Kamisato/internal/pacman/source"
	"github.com/Hayao0819/Kamisato/pkg/raiou"
)

func TestPruneRemovesRemotePackagesMissingFromSource(t *testing.T) {
	src := pruneSource(t)
	remote := &repo.RemoteRepo{Pkgs: []*pkg.BinaryPackage{
		pruneBinary("foo"),
		pruneBinary("foo-docs"),
		pruneBinary("orphan"),
	}}

	var removed []string
	packages, err := prune(context.Background(), src, PruneOptions{
		Arch: "x86_64",
		Remove: func(_ context.Context, repository, name string) error {
			removed = append(removed, repository+"/"+name)
			return nil
		},
	}, func(url, name string) (*repo.RemoteRepo, error) {
		if url != "https://repo.example/core/x86_64" {
			t.Errorf("url = %q", url)
		}
		if name != "core" {
			t.Errorf("name = %q", name)
		}
		return remote, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"orphan"}; !reflect.DeepEqual(packages, want) {
		t.Errorf("packages = %v, want %v", packages, want)
	}
	if want := []string{"core/orphan"}; !reflect.DeepEqual(removed, want) {
		t.Errorf("removed = %v, want %v", removed, want)
	}
}

func TestPruneDryRunDoesNotRequireRemover(t *testing.T) {
	packages, err := prune(context.Background(), pruneSource(t), PruneOptions{
		Arch:   "x86_64",
		DryRun: true,
	}, func(_, _ string) (*repo.RemoteRepo, error) {
		return &repo.RemoteRepo{Pkgs: []*pkg.BinaryPackage{pruneBinary("orphan")}}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"orphan"}; !reflect.DeepEqual(packages, want) {
		t.Errorf("packages = %v, want %v", packages, want)
	}
}

func TestPruneRefusesEmptySource(t *testing.T) {
	fetched := false
	_, err := prune(context.Background(), &source.SourceRepo{
		Config: &source.SrcConfig{Name: "core", URL: "https://repo.example/core"},
	}, PruneOptions{Arch: "x86_64"}, func(_, _ string) (*repo.RemoteRepo, error) {
		fetched = true
		return nil, nil
	})
	if err == nil {
		t.Fatal("empty source should be rejected")
	}
	if fetched {
		t.Error("remote repository was fetched before validating the source package set")
	}
}

func TestPruneTreatsMissingRemoteRepositoryAsEmpty(t *testing.T) {
	packages, err := prune(context.Background(), pruneSource(t), PruneOptions{
		Arch: "x86_64",
	}, func(_, _ string) (*repo.RemoteRepo, error) {
		return nil, stderrors.Join(repo.ErrRepoNotFound, stderrors.New("missing"))
	})
	if err != nil {
		t.Fatal(err)
	}
	if packages != nil {
		t.Errorf("packages = %v, want nil", packages)
	}
}

func pruneSource(t *testing.T) *source.SourceRepo {
	t.Helper()
	return &source.SourceRepo{
		Config: &source.SrcConfig{Name: "core", URL: "https://repo.example/core/"},
		Pkgs: []*pkg.SourcePackage{srcinfoPkg(t, `pkgbase = foo
	pkgver = 1
	pkgrel = 1
	arch = x86_64

pkgname = foo

pkgname = foo-docs
`)},
	}
}

func pruneBinary(name string) *pkg.BinaryPackage {
	return pkg.NewBinaryPackage(name+"-1-1-x86_64.pkg.tar.zst", &raiou.PKGINFO{
		PkgBase: name,
		PkgName: name,
		PkgVer:  "1-1",
	})
}
