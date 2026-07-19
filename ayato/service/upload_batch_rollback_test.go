package service_test

import (
	"testing"

	"github.com/Hayao0819/Kamisato/internal/errors"
	"go.uber.org/mock/gomock"

	"github.com/Hayao0819/Kamisato/ayato/domain"
	"github.com/Hayao0819/Kamisato/ayato/platform"
	"github.com/Hayao0819/Kamisato/ayato/repository"
	"github.com/Hayao0819/Kamisato/ayato/repository/blob"
	"github.com/Hayao0819/Kamisato/ayato/service"
	"github.com/Hayao0819/Kamisato/ayato/test/mocks"
	"github.com/Hayao0819/Kamisato/internal/conf"
	pkgpkg "github.com/Hayao0819/Kamisato/pkg/pacman/pkg"
	"github.com/Hayao0819/Kamisato/pkg/pacman/repo"
	"github.com/Hayao0819/Kamisato/pkg/raiou"
)

func TestUploadFiles_SecondArchFailureRestoresFirstArch(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	bin := mocks.NewMockBinaryRepository(ctrl)
	name := mocks.NewMockNameStore(ctrl)
	cfg := &conf.AyatoConfig{Repos: []conf.BinRepoConfig{{
		Name: "myrepo", Arches: []string{"x86_64", "aarch64"},
	}}}
	oldName := "foo-0.9-1-any.pkg.tar.zst"
	oldRepo := &repo.RemoteRepo{Pkgs: []*pkgpkg.BinaryPackage{
		pkgpkg.NewBinaryPackage(
			oldName,
			&raiou.PKGINFO{PkgName: "foo", PkgVer: "0.9-1", Arch: "any"},
		),
	}}
	bin.EXPECT().VerifyPkgRepo("myrepo").Return(nil)
	bin.EXPECT().Arches("myrepo").Return([]string{"x86_64", "aarch64"}, nil).AnyTimes()
	bin.EXPECT().RemoteRepo("myrepo", "x86_64").Return(oldRepo, nil)
	bin.EXPECT().RemoteRepo("myrepo", "aarch64").Return(oldRepo, nil)
	bin.EXPECT().FetchFile("myrepo", "any", oldName).
		Return(pkgStream(oldName, buildPackage(t, "foo", "0.9-1", "any")), nil)
	bin.EXPECT().FetchFile("myrepo", "any", oldName+".sig").Return(nil, blob.ErrNotFound)

	storeCalls := 0
	bin.EXPECT().StoreFileImmutable("myrepo", "any", gomock.Any()).DoAndReturn(
		func(_ string, _ string, file platform.SeekFile) (bool, error) {
			storeCalls++
			want := "foo-1.0-1-any.pkg.tar.zst"
			if storeCalls == 2 {
				want = oldName
			}
			if file.FileName() != want {
				t.Fatalf("StoreFileImmutable call %d stored %q, want %q", storeCalls, file.FileName(), want)
			}
			return true, nil
		},
	).Times(2)

	x86Calls := 0
	bin.EXPECT().RepoAddBatch("myrepo", "x86_64", gomock.Any(), false, gomock.Nil()).DoAndReturn(
		func(_ string, _ string, items []repository.RepoAddItem, _ bool, _ *string) error {
			x86Calls++
			want := "foo-1.0-1-any.pkg.tar.zst"
			if x86Calls == 2 {
				want = oldName
			}
			if len(items) != 1 || items[0].Pkg.FileName() != want {
				t.Fatalf("x86_64 RepoAddBatch call %d file = %v, want %s", x86Calls, items, want)
			}
			return nil
		},
	).Times(2)
	bin.EXPECT().RepoAddBatch("myrepo", "aarch64", gomock.Any(), false, gomock.Nil()).
		Return(errors.New("aarch64 commit failed"))

	svc := service.New(name, bin, nil, nil, cfg)
	files := []*domain.UploadFiles{{
		PkgFile: pkgStream(
			"foo-1.0-1-any.pkg.tar.zst",
			buildPackage(t, "foo", "1.0-1", "any"),
		),
	}}
	if err := svc.UploadFiles("myrepo", files); err == nil {
		t.Fatal("expected second-arch commit failure")
	}
	if x86Calls != 2 || storeCalls != 2 {
		t.Fatalf("restore calls = db:%d store:%d, want 2 each", x86Calls, storeCalls)
	}
}

func TestUploadFiles_PostCanonicalSupersessionCompensatesOwnedPackages(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	bin := mocks.NewMockBinaryRepository(ctrl)
	name := mocks.NewMockNameStore(ctrl)
	bin.EXPECT().VerifyPkgRepo("myrepo").Return(nil)
	bin.EXPECT().RemoteRepo("myrepo", "x86_64").Return(&repo.RemoteRepo{}, nil).AnyTimes()
	bin.EXPECT().StoreFileImmutable("myrepo", "x86_64", gomock.Any()).Return(true, nil).Times(2)
	bin.EXPECT().RepoAddBatch("myrepo", "x86_64", gomock.Any(), false, gomock.Nil()).
		Return(&repository.CanonicalCommitError{Err: repository.ErrPackageChanged})
	bin.EXPECT().RepoRemoveIfMatch(
		"myrepo", "x86_64", "foo", "1.0-1", "foo-1.0-1-x86_64.pkg.tar.zst", false, gomock.Nil(),
	).Return(repository.ErrPackageChanged)
	bin.EXPECT().RepoRemoveIfMatch(
		"myrepo", "x86_64", "bar", "1.0-1", "bar-1.0-1-x86_64.pkg.tar.zst", false, gomock.Nil(),
	).Return(nil)
	bin.EXPECT().ReconcileDB("myrepo", "x86_64", false, gomock.Nil()).Return(nil)

	svc := service.New(name, bin, nil, nil, baseConfig(false, ""))
	err := svc.UploadFiles("myrepo", []*domain.UploadFiles{
		{PkgFile: pkgStream("foo-1.0-1-x86_64.pkg.tar.zst", buildPackage(t, "foo", "1.0-1", "x86_64"))},
		{PkgFile: pkgStream("bar-1.0-1-x86_64.pkg.tar.zst", buildPackage(t, "bar", "1.0-1", "x86_64"))},
	})
	if !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("superseded batch = %v, want ErrConflict", err)
	}
}
