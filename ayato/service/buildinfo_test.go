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

func provenanceConfig() *conf.AyatoConfig {
	cfg := &conf.AyatoConfig{
		RequireBuildinfoProvenance: true,
		Repos:                      []conf.BinRepoConfig{{Name: "myrepo", Arches: []string{"x86_64"}}},
	}
	return cfg
}

func TestUploadFile_BuildinfoProvenanceAccepts(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	bin := mocks.NewMockBinaryRepository(ctrl)
	name := mocks.NewMockNameStore(ctrl)
	bin.EXPECT().VerifyPkgRepo("myrepo").Return(nil)
	bin.EXPECT().RemoteRepo("myrepo", "x86_64").Return(&repo.RemoteRepo{}, nil).AnyTimes()
	bin.EXPECT().StoreFileImmutable("myrepo", "x86_64", gomock.Any()).Return(true, nil)
	bin.EXPECT().RepoAddBatch("myrepo", "x86_64", gomock.Any(), false, gomock.Nil()).Return(nil)
	name.EXPECT().StorePackageFiles("myrepo", []repository.PackageFileEntry{{Arch: "x86_64", Name: "foo", FileName: uploadName}}).Return(nil)

	svc := service.New(name, bin, nil, nil, provenanceConfig())
	files := &domain.UploadFiles{PkgFile: pkgStream(uploadName, buildPackage(
		t,
		"foo",
		"1.0-1",
		"x86_64",
		withPackageMember(".BUILDINFO", "format = 2\nbuilddir = /build\n"),
	))}
	if err := svc.UploadFile("myrepo", files); err != nil {
		t.Fatalf("a package built in /build must pass provenance: %v", err)
	}
}

func TestUploadFile_BuildinfoProvenanceRejectsInvalidPackages(t *testing.T) {
	for _, test := range []struct {
		name    string
		options []packageOption
	}{
		{
			name: "foreign build directory",
			options: []packageOption{
				withPackageMember(".BUILDINFO", "format = 2\nbuilddir = /home/mallory/x\n"),
			},
		},
		{name: "missing buildinfo"},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			bin := mocks.NewMockBinaryRepository(ctrl)
			name := mocks.NewMockNameStore(ctrl)
			bin.EXPECT().VerifyPkgRepo("myrepo").Return(nil)

			svc := service.New(name, bin, nil, nil, provenanceConfig())
			files := &domain.UploadFiles{PkgFile: pkgStream(
				uploadName,
				buildPackage(t, "foo", "1.0-1", "x86_64", test.options...),
			)}
			err := svc.UploadFile("myrepo", files)
			if !errors.Is(err, domain.ErrInvalidUpload) {
				t.Fatalf("%s = %v, want ErrInvalidUpload", test.name, err)
			}
		})
	}
}
