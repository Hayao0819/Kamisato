package service_test

import (
	"bytes"
	"testing"

	"github.com/Hayao0819/Kamisato/internal/errors"
	"go.uber.org/mock/gomock"

	"github.com/Hayao0819/Kamisato/ayato/domain"
	"github.com/Hayao0819/Kamisato/ayato/repository"
	"github.com/Hayao0819/Kamisato/ayato/repository/blob"
	"github.com/Hayao0819/Kamisato/ayato/service"
	"github.com/Hayao0819/Kamisato/ayato/stream"
	"github.com/Hayao0819/Kamisato/ayato/test/mocks"
	"github.com/Hayao0819/Kamisato/internal/conf"
	pkgpkg "github.com/Hayao0819/Kamisato/pkg/pacman/pkg"
	"github.com/Hayao0819/Kamisato/pkg/pacman/repo"
	"github.com/Hayao0819/Kamisato/pkg/raiou"
)

func remoteWith(pkgname, version string) *repo.RemoteRepo {
	return &repo.RemoteRepo{Pkgs: []*pkgpkg.BinaryPackage{
		pkgpkg.NewBinaryPackage(
			pkgname+"-"+version+"-x86_64.pkg.tar.zst",
			&raiou.PKGINFO{PkgName: pkgname, Arch: "x86_64", PkgVer: version},
		),
	}}
}

func pkgStream(name string, data []byte) *stream.FileStream {
	return stream.NewFileStream(
		name,
		"application/octet-stream",
		bufferToReadSeekCloser(bytes.NewBuffer(data)),
	)
}

const uploadName = "foo-1.0-1-x86_64.pkg.tar.zst"

func baseConfig(requireSign bool, keyring string) *conf.AyatoConfig {
	cfg := &conf.AyatoConfig{
		RequireSign: requireSign,
		Repos:       []conf.BinRepoConfig{{Name: "myrepo", Arches: []string{"x86_64"}}},
	}
	cfg.Verify.Keyring = keyring
	return cfg
}

func TestUploadFile_RejectsNonUpgrade(t *testing.T) {
	for _, test := range []struct {
		name           string
		currentVersion string
	}{
		{name: "downgrade", currentVersion: "2.0-1"},
		{name: "duplicate", currentVersion: "1.0-1"},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			bin := mocks.NewMockBinaryRepository(ctrl)
			name := mocks.NewMockNameStore(ctrl)
			bin.EXPECT().VerifyPkgRepo("myrepo").Return(nil)
			bin.EXPECT().RemoteRepo("myrepo", "x86_64").
				Return(remoteWith("foo", test.currentVersion), nil)

			svc := service.New(name, bin, nil, nil, baseConfig(false, ""))
			files := &domain.UploadFiles{
				PkgFile: pkgStream(uploadName, buildPackage(t, "foo", "1.0-1", "x86_64")),
			}
			err := svc.UploadFile("myrepo", files)
			if !errors.Is(err, domain.ErrInvalidUpload) {
				t.Fatalf("%s = %v, want ErrInvalidUpload", test.name, err)
			}
		})
	}
}

func TestUploadFile_FilenameMustMatchPackageMetadata(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	bin := mocks.NewMockBinaryRepository(ctrl)
	name := mocks.NewMockNameStore(ctrl)
	bin.EXPECT().VerifyPkgRepo("myrepo").Return(nil)

	svc := service.New(name, bin, nil, nil, baseConfig(false, ""))
	err := svc.UploadFile("myrepo", &domain.UploadFiles{
		PkgFile: pkgStream(uploadName, buildPackage(t, "evil", "1.0-1", "x86_64")),
	})
	if !errors.Is(err, domain.ErrInvalidUpload) {
		t.Fatalf("filename/metadata mismatch = %v, want ErrInvalidUpload", err)
	}
}

func TestUploadFile_UpgradeAccepted(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	bin := mocks.NewMockBinaryRepository(ctrl)
	name := mocks.NewMockNameStore(ctrl)
	oldName := "foo-0.9-1-x86_64.pkg.tar.zst"
	bin.EXPECT().VerifyPkgRepo("myrepo").Return(nil)
	bin.EXPECT().RemoteRepo("myrepo", "x86_64").Return(remoteWith("foo", "0.9-1"), nil)
	bin.EXPECT().FetchFile("myrepo", "x86_64", oldName).
		Return(pkgStream(oldName, buildPackage(t, "foo", "0.9-1", "x86_64")), nil)
	bin.EXPECT().FetchFile("myrepo", "x86_64", oldName+".sig").Return(nil, blob.ErrNotFound)
	bin.EXPECT().StoreFileImmutable("myrepo", "x86_64", gomock.Any()).Return(true, nil)
	bin.EXPECT().RepoAddBatch("myrepo", "x86_64", gomock.Any(), false, gomock.Nil()).Return(nil)
	name.EXPECT().StorePackageFiles("myrepo", []repository.PackageFileEntry{
		{Arch: "x86_64", Name: "foo", FileName: uploadName},
	}).Return(nil)

	svc := service.New(name, bin, nil, nil, baseConfig(false, ""))
	files := &domain.UploadFiles{
		PkgFile: pkgStream(uploadName, buildPackage(t, "foo", "1.0-1", "x86_64")),
	}
	if err := svc.UploadFile("myrepo", files); err != nil {
		t.Fatalf("an upgrade should be accepted: %v", err)
	}
}
