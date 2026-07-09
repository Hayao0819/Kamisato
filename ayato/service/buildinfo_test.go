package service_test

import (
	"archive/tar"
	"bytes"
	"testing"

	"github.com/Hayao0819/Kamisato/internal/errors"

	"github.com/klauspost/compress/zstd"
	"go.uber.org/mock/gomock"

	"github.com/Hayao0819/Kamisato/ayato/domain"
	"github.com/Hayao0819/Kamisato/ayato/service"
	"github.com/Hayao0819/Kamisato/ayato/test/mocks"
	"github.com/Hayao0819/Kamisato/internal/conf"
	"github.com/Hayao0819/Kamisato/pkg/pacman/repo"
)

// buildPkgWithBuildinfo returns a foo-1.0-1 .pkg.tar.zst; buildDir seeds the
// .BUILDINFO builddir, and includeBuildinfo omits the member entirely when false.
func buildPkgWithBuildinfo(t *testing.T, buildDir string, includeBuildinfo bool) []byte {
	t.Helper()
	var tarBuf bytes.Buffer
	tw := tar.NewWriter(&tarBuf)
	write := func(name, body string) {
		if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o644, Size: int64(len(body))}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	write(".PKGINFO", pkginfoBody)
	if includeBuildinfo {
		write(".BUILDINFO", "format = 2\nbuilddir = "+buildDir+"\n")
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	var zBuf bytes.Buffer
	zw, err := zstd.NewWriter(&zBuf)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := zw.Write(tarBuf.Bytes()); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return zBuf.Bytes()
}

func provenanceConfig() *conf.AyatoConfig {
	cfg := &conf.AyatoConfig{
		RequireBuildinfoProvenance: true,
		Repos:                      []conf.BinRepoConfig{{Name: "myrepo"}},
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
	bin.EXPECT().StoreFile("myrepo", "x86_64", gomock.Any()).Return(nil)
	bin.EXPECT().RepoAddBatch("myrepo", "x86_64", gomock.Any(), false, gomock.Nil()).Return(nil)
	name.EXPECT().StorePackageFile("x86_64", "foo", uploadName).Return(nil)

	svc := service.New(name, bin, nil, nil, provenanceConfig())
	files := &domain.UploadFiles{PkgFile: pkgStream(uploadName, buildPkgWithBuildinfo(t, "/build", true))}
	if err := svc.UploadFile("myrepo", files); err != nil {
		t.Fatalf("a package built in /build must pass provenance: %v", err)
	}
}

func TestUploadFile_BuildinfoProvenanceRejectsForeignBuildDir(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	bin := mocks.NewMockBinaryRepository(ctrl)
	name := mocks.NewMockNameStore(ctrl)
	bin.EXPECT().VerifyPkgRepo("myrepo").Return(nil)
	// No StoreFile / RepoAddBatch: the gate rejects before anything is stored.

	svc := service.New(name, bin, nil, nil, provenanceConfig())
	files := &domain.UploadFiles{PkgFile: pkgStream(uploadName, buildPkgWithBuildinfo(t, "/home/mallory/x", true))}
	err := svc.UploadFile("myrepo", files)
	if !errors.Is(err, domain.ErrInvalidUpload) {
		t.Fatalf("a foreign builddir must be a client error (ErrInvalidUpload), got %v", err)
	}
}

func TestUploadFile_BuildinfoProvenanceRejectsMissing(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	bin := mocks.NewMockBinaryRepository(ctrl)
	name := mocks.NewMockNameStore(ctrl)
	bin.EXPECT().VerifyPkgRepo("myrepo").Return(nil)

	svc := service.New(name, bin, nil, nil, provenanceConfig())
	files := &domain.UploadFiles{PkgFile: pkgStream(uploadName, buildPkgWithBuildinfo(t, "", false))}
	err := svc.UploadFile("myrepo", files)
	if !errors.Is(err, domain.ErrInvalidUpload) {
		t.Fatalf("a missing .BUILDINFO with the gate on must be rejected (ErrInvalidUpload), got %v", err)
	}
}
