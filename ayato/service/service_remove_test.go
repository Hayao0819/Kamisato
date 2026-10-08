package service_test

import (
	"errors"
	"testing"

	"go.uber.org/mock/gomock"

	ayatoconfig "github.com/Hayao0819/Kamisato/ayato/config"
	"github.com/Hayao0819/Kamisato/ayato/service"
	"github.com/Hayao0819/Kamisato/ayato/test/mocks"
	pkgpkg "github.com/Hayao0819/Kamisato/internal/pacman/pkg"
	"github.com/Hayao0819/Kamisato/internal/pacman/repo"
	"github.com/Hayao0819/Kamisato/pkg/raiou"
)

func anyPackageRepo() *repo.RemoteRepo {
	return &repo.RemoteRepo{Pkgs: []*pkgpkg.BinaryPackage{
		pkgpkg.NewBinaryPackage(
			"mypkg-1.0-1-any.pkg.tar.zst",
			&raiou.PKGINFO{PkgName: "mypkg", Arch: "any"},
		),
	}}
}

func TestServiceRemovePkg(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	bin := mocks.NewMockBinaryRepository(ctrl)
	names := mocks.NewMockNameStore(ctrl)
	bin.EXPECT().RepoNames().Return([]string{"myrepo"}, nil)
	bin.EXPECT().Arches("myrepo").Return([]string{"x86_64"}, nil).AnyTimes()
	bin.EXPECT().RemoteRepo("myrepo", "x86_64").Return(&repo.RemoteRepo{
		Pkgs: []*pkgpkg.BinaryPackage{
			pkgpkg.NewBinaryPackage(
				"mypkg-1.0-1-x86_64.pkg.tar.zst",
				&raiou.PKGINFO{PkgName: "mypkg", Arch: "x86_64"},
			),
		},
	}, nil)
	names.EXPECT().StorePackageFile(
		"myrepo", "x86_64", "mypkg", "mypkg-1.0-1-x86_64.pkg.tar.zst",
	).Return(nil)
	bin.EXPECT().RepoRemove("myrepo", "x86_64", "mypkg", false, gomock.Nil()).Return(nil)
	bin.EXPECT().DeleteFile(
		"myrepo", "x86_64", "mypkg-1.0-1-x86_64.pkg.tar.zst",
	).Return(nil)
	bin.EXPECT().Files("myrepo", "x86_64").
		Return([]string{"mypkg-1.0-1-x86_64.pkg.tar.zst", "myrepo.db"}, nil)
	names.EXPECT().DeletePackageFileEntry("myrepo", "x86_64", "mypkg").Return(nil)

	svc := service.New(names, bin, nil, nil, repoConfig())
	if err := svc.RemovePkg("myrepo", "x86_64", "mypkg"); err != nil {
		t.Fatalf("RemovePkg failed: %v", err)
	}
}

func TestServiceRemoveAnyPackageFromAllArches(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	bin := mocks.NewMockBinaryRepository(ctrl)
	names := mocks.NewMockNameStore(ctrl)
	bin.EXPECT().RepoNames().Return([]string{"myrepo"}, nil)
	bin.EXPECT().Arches("myrepo").Return([]string{"x86_64", "aarch64"}, nil).AnyTimes()
	bin.EXPECT().RemoteRepo("myrepo", "x86_64").Return(anyPackageRepo(), nil)
	names.EXPECT().StorePackageFile(
		"myrepo", "any", "mypkg", "mypkg-1.0-1-any.pkg.tar.zst",
	).Return(nil)
	bin.EXPECT().RepoRemove("myrepo", "x86_64", "mypkg", false, gomock.Nil()).Return(nil)
	bin.EXPECT().RepoRemove("myrepo", "aarch64", "mypkg", false, gomock.Nil()).Return(nil)
	bin.EXPECT().DeleteFile("myrepo", "any", "mypkg-1.0-1-any.pkg.tar.zst").Return(nil)
	bin.EXPECT().Files("myrepo", "any").Return([]string{
		"mypkg-1.0-1-any.pkg.tar.zst",
		"mypkg-1.0-1-any.pkg.tar.zst.sig",
	}, nil)
	bin.EXPECT().DeleteFile("myrepo", "any", "mypkg-1.0-1-any.pkg.tar.zst.sig").Return(nil)
	names.EXPECT().DeletePackageFileEntry("myrepo", "any", "mypkg").Return(nil)

	svc := service.New(names, bin, nil, nil, repoConfig())
	if err := svc.RemovePkg("myrepo", "", "mypkg"); err != nil {
		t.Fatalf("RemovePkg(any, all arches) failed: %v", err)
	}
}

func TestServiceRemoveAnyPackageFromOneArchKeepsSharedFile(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	bin := mocks.NewMockBinaryRepository(ctrl)
	names := mocks.NewMockNameStore(ctrl)
	bin.EXPECT().RepoNames().Return([]string{"myrepo"}, nil)
	bin.EXPECT().Arches("myrepo").Return([]string{"x86_64", "aarch64"}, nil).AnyTimes()
	bin.EXPECT().RemoteRepo("myrepo", "x86_64").Return(anyPackageRepo(), nil)
	names.EXPECT().StorePackageFile(
		"myrepo", "any", "mypkg", "mypkg-1.0-1-any.pkg.tar.zst",
	).Return(nil)
	bin.EXPECT().RepoRemove("myrepo", "x86_64", "mypkg", false, gomock.Nil()).Return(nil)
	bin.EXPECT().RemoteRepo("myrepo", "aarch64").Return(anyPackageRepo(), nil)

	svc := service.New(names, bin, nil, nil, repoConfig())
	if err := svc.RemovePkg("myrepo", "x86_64", "mypkg"); err != nil {
		t.Fatalf("RemovePkg(any one arch) failed: %v", err)
	}
}

func TestServiceRemoveAnyPackageKeepsObjectsWhenOtherArchesCannotBeVerified(t *testing.T) {
	for _, failure := range []string{"list arches", "read other arch"} {
		t.Run(failure, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			bin := mocks.NewMockBinaryRepository(ctrl)
			names := mocks.NewMockNameStore(ctrl)
			want := errors.New("repository unavailable")
			bin.EXPECT().RepoNames().Return([]string{"myrepo"}, nil)
			// Source resolution's best-effort list is not authority for deletion.
			firstList := bin.EXPECT().Arches("myrepo").Return([]string{"x86_64", "aarch64"}, nil)
			secondList := bin.EXPECT().Arches("myrepo")
			if failure == "list arches" {
				secondList.Return(nil, want)
			} else {
				secondList.Return([]string{"x86_64", "aarch64"}, nil)
				bin.EXPECT().RepoRemove("myrepo", "x86_64", "mypkg", false, gomock.Nil()).Return(nil)
				bin.EXPECT().RemoteRepo("myrepo", "aarch64").Return(nil, want)
			}
			gomock.InOrder(firstList, secondList)
			bin.EXPECT().RemoteRepo("myrepo", "x86_64").Return(anyPackageRepo(), nil)
			names.EXPECT().StorePackageFile("myrepo", "any", "mypkg", "mypkg-1.0-1-any.pkg.tar.zst").Return(nil)
			// No DeleteFile, signature deletion, or metadata deletion is allowed.
			svc := service.New(names, bin, nil, nil, repoConfig())
			if err := svc.RemovePkg("myrepo", "x86_64", "mypkg"); !errors.Is(err, want) {
				t.Fatalf("RemovePkg = %v, want backend error", err)
			}
		})
	}
}

func repoConfig() service.Settings {
	return settingsFromConfig(&ayatoconfig.AyatoConfig{Repos: []ayatoconfig.BinRepoConfig{{Name: "myrepo"}}})
}
