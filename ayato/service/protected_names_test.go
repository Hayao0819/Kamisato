package service_test

import (
	"testing"

	"github.com/Hayao0819/Kamisato/internal/errors"

	"go.uber.org/mock/gomock"

	"github.com/Hayao0819/Kamisato/ayato/domain"
	"github.com/Hayao0819/Kamisato/ayato/repository"
	"github.com/Hayao0819/Kamisato/ayato/service"
	"github.com/Hayao0819/Kamisato/ayato/test/mocks"
	"github.com/Hayao0819/Kamisato/internal/conf"
	"github.com/Hayao0819/Kamisato/pkg/pacman/repo"
)

func protectedConfig(names ...string) *conf.AyatoConfig {
	return &conf.AyatoConfig{
		ProtectedNames: names,
		Repos:          []conf.BinRepoConfig{{Name: "myrepo", Arches: []string{"x86_64"}}},
	}
}

func TestUploadFile_ProtectedNameCollision(t *testing.T) {
	cases := []struct {
		name    string
		pkgname string
		extra   string
	}{
		{"pkgname", "pacman", ""},
		{"provides", "evil", "provides = glibc=2.39\n"},
		{"replaces", "evil", "replaces = pacman\n"},
		{"groups", "evil", "group = base\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			bin := mocks.NewMockBinaryRepository(ctrl)
			name := mocks.NewMockNameStore(ctrl)
			bin.EXPECT().VerifyPkgRepo("myrepo").Return(nil)
			// No StoreFile / RepoAddBatch: the guard rejects before anything is stored.

			svc := service.New(name, bin, nil, nil, protectedConfig("pacman", "glibc", "base"))
			fileName := tc.pkgname + "-1.0-1-x86_64.pkg.tar.zst"
			files := &domain.UploadFiles{PkgFile: pkgStream(
				fileName,
				buildPackage(t, tc.pkgname, "1.0-1", "x86_64", withPKGINFO(tc.extra)),
			)}
			err := svc.UploadFile("myrepo", files)
			if !errors.Is(err, domain.ErrConflict) {
				t.Fatalf("a protected-name collision must be ErrConflict, got %v", err)
			}
		})
	}
}

func TestUploadFile_ProtectedNamesAllowNonCollision(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	bin := mocks.NewMockBinaryRepository(ctrl)
	name := mocks.NewMockNameStore(ctrl)
	bin.EXPECT().VerifyPkgRepo("myrepo").Return(nil)
	bin.EXPECT().RemoteRepo("myrepo", "x86_64").Return(&repo.RemoteRepo{}, nil).AnyTimes()
	bin.EXPECT().StoreFileImmutable("myrepo", "x86_64", gomock.Any()).Return(true, nil)
	bin.EXPECT().RepoAddBatch("myrepo", "x86_64", gomock.Any(), false, gomock.Nil()).Return(nil)
	name.EXPECT().StorePackageFiles("myrepo", []repository.PackageFileEntry{{Arch: "x86_64", Name: "foo", FileName: uploadName}}).Return(nil)

	svc := service.New(name, bin, nil, nil, protectedConfig("pacman", "glibc"))
	// foo provides a lookalike-but-distinct name; nothing collides with the list.
	files := &domain.UploadFiles{PkgFile: pkgStream(
		uploadName,
		buildPackage(t, "foo", "1.0-1", "x86_64", withPKGINFO("provides = libfoo.so=1-64\n")),
	)}
	if err := svc.UploadFile("myrepo", files); err != nil {
		t.Fatalf("a non-colliding package must be accepted: %v", err)
	}
}
