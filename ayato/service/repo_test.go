package service_test

import (
	"testing"

	"go.uber.org/mock/gomock"

	"github.com/Hayao0819/Kamisato/ayato/domain"
	"github.com/Hayao0819/Kamisato/ayato/service"
	"github.com/Hayao0819/Kamisato/ayato/test/mocks"
	"github.com/Hayao0819/Kamisato/internal/conf"
	"github.com/Hayao0819/Kamisato/internal/errors"
	pacmanpkg "github.com/Hayao0819/Kamisato/pkg/pacman/pkg"
	"github.com/Hayao0819/Kamisato/pkg/pacman/repo"
	"github.com/Hayao0819/Kamisato/pkg/raiou"
)

func TestPackageQueriesIncludeCanonicalFilename(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	binaryRepo := mocks.NewMockBinaryRepository(ctrl)
	nameRepo := mocks.NewMockNameStore(ctrl)
	const filename = "demo-2.0-3-any.pkg.tar.xz"
	remote := &repo.RemoteRepo{Pkgs: []*pacmanpkg.BinaryPackage{
		pacmanpkg.NewBinaryPackage(filename, &raiou.PKGINFO{
			PkgName: "demo",
			PkgBase: "demo",
			PkgVer:  "2.0-3",
			Arch:    "any",
		}),
	}}
	binaryRepo.EXPECT().RemoteRepo("core", "x86_64").Return(remote, nil).Times(2)

	svc := service.New(nameRepo, binaryRepo, nil, nil, &conf.AyatoConfig{})
	packages, err := svc.Pkgs("core", "x86_64")
	if err != nil {
		t.Fatal(err)
	}
	if len(packages.Packages) != 1 || packages.Packages[0].Filename != filename {
		t.Fatalf("packages = %+v, want canonical filename %q", packages.Packages, filename)
	}
	detail, err := svc.PkgDetail("core", "x86_64", "demo")
	if err != nil {
		t.Fatal(err)
	}
	if detail.Filename != filename {
		t.Errorf("detail filename = %q, want %q", detail.Filename, filename)
	}
}

func TestPkgDetailUsesPackageNameForSplitPackages(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	binaryRepo := mocks.NewMockBinaryRepository(ctrl)
	nameRepo := mocks.NewMockNameStore(ctrl)
	const filename = "demo-headers-2.0-3-any.pkg.tar.zst"
	remote := &repo.RemoteRepo{Pkgs: []*pacmanpkg.BinaryPackage{
		pacmanpkg.NewBinaryPackage(filename, &raiou.PKGINFO{
			PkgName: "demo-headers",
			PkgBase: "demo",
			PkgVer:  "2.0-3",
			Arch:    "any",
		}),
	}}
	binaryRepo.EXPECT().RemoteRepo("core", "x86_64").Return(remote, nil).Times(2)

	svc := service.New(nameRepo, binaryRepo, nil, nil, &conf.AyatoConfig{})
	detail, err := svc.PkgDetail("core", "x86_64", "demo-headers")
	if err != nil {
		t.Fatal(err)
	}
	if detail.PkgName != "demo-headers" || detail.PkgBase != "demo" {
		t.Fatalf("detail = %+v, want split package demo-headers (base demo)", detail)
	}

	_, err = svc.PkgDetail("core", "x86_64", "demo")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("PkgDetail by pkgbase error = %v, want ErrNotFound", err)
	}
}
