package service_test

import (
	"testing"

	"github.com/Hayao0819/Kamisato/internal/errors"
	"go.uber.org/mock/gomock"

	"github.com/Hayao0819/Kamisato/ayato/domain"
	"github.com/Hayao0819/Kamisato/ayato/repository"
	"github.com/Hayao0819/Kamisato/ayato/service"
	"github.com/Hayao0819/Kamisato/ayato/test/mocks"
	"github.com/Hayao0819/Kamisato/pkg/pacman/repo"
)

type countingSignerRepo struct{ listCalls int }

func (r *countingSignerRepo) AddSigner(string, []byte) error { return nil }
func (r *countingSignerRepo) DeleteSigner(string) error      { return nil }
func (r *countingSignerRepo) ListSigners() ([][]byte, error) {
	r.listCalls++
	return nil, nil
}

func TestUploadFiles_KeyringBuiltOncePerBatch(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	signer := newSigner(t)
	keyring := writeKeyring(t, signer)
	fooPayload := buildPackage(t, "foo", "1.0-1", "x86_64")
	barPayload := buildPackage(t, "bar", "1.0-1", "x86_64")

	bin := mocks.NewMockBinaryRepository(ctrl)
	name := mocks.NewMockNameStore(ctrl)
	bin.EXPECT().VerifyPkgRepo("myrepo").Return(nil)
	bin.EXPECT().RemoteRepo("myrepo", "x86_64").Return(&repo.RemoteRepo{}, nil).AnyTimes()
	bin.EXPECT().StoreFileImmutable("myrepo", "x86_64", gomock.Any()).Return(true, nil).Times(4)
	bin.EXPECT().RepoAddBatch("myrepo", "x86_64", gomock.Any(), false, gomock.Nil()).Return(nil)
	name.EXPECT().StorePackageFiles("myrepo", []repository.PackageFileEntry{
		{Arch: "x86_64", Name: "foo", FileName: "foo-1.0-1-x86_64.pkg.tar.zst"},
		{Arch: "x86_64", Name: "bar", FileName: "bar-1.0-1-x86_64.pkg.tar.zst"},
	}).Return(nil)

	signers := &countingSignerRepo{}
	svc := service.New(name, bin, nil, signers, baseConfig(false, keyring))
	files := []*domain.UploadFiles{
		{
			PkgFile: pkgStream("foo-1.0-1-x86_64.pkg.tar.zst", fooPayload),
			SigFile: pkgStream("foo-1.0-1-x86_64.pkg.tar.zst.sig", detachSignBytes(t, signer, fooPayload)),
		},
		{
			PkgFile: pkgStream("bar-1.0-1-x86_64.pkg.tar.zst", barPayload),
			SigFile: pkgStream("bar-1.0-1-x86_64.pkg.tar.zst.sig", detachSignBytes(t, signer, barPayload)),
		},
	}
	if err := svc.UploadFiles("myrepo", files); err != nil {
		t.Fatalf("UploadFiles: %v", err)
	}
	if signers.listCalls != 1 {
		t.Fatalf("signer keyring rebuilt %d times, want 1", signers.listCalls)
	}
}

func TestUploadFiles_BatchOneRepoAddPerArch(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	bin := mocks.NewMockBinaryRepository(ctrl)
	name := mocks.NewMockNameStore(ctrl)
	bin.EXPECT().VerifyPkgRepo("myrepo").Return(nil)
	bin.EXPECT().RemoteRepo("myrepo", "x86_64").Return(&repo.RemoteRepo{}, nil).AnyTimes()
	bin.EXPECT().StoreFileImmutable("myrepo", "x86_64", gomock.Any()).Return(true, nil).Times(2)
	bin.EXPECT().RepoAddBatch("myrepo", "x86_64", gomock.Any(), false, gomock.Nil()).Return(nil)
	name.EXPECT().StorePackageFiles("myrepo", []repository.PackageFileEntry{
		{Arch: "x86_64", Name: "foo", FileName: "foo-1.0-1-x86_64.pkg.tar.zst"},
		{Arch: "x86_64", Name: "bar", FileName: "bar-1.0-1-x86_64.pkg.tar.zst"},
	}).Return(nil)

	svc := service.New(name, bin, nil, nil, baseConfig(false, ""))
	files := []*domain.UploadFiles{
		{PkgFile: pkgStream("foo-1.0-1-x86_64.pkg.tar.zst", buildPackage(t, "foo", "1.0-1", "x86_64"))},
		{PkgFile: pkgStream("bar-1.0-1-x86_64.pkg.tar.zst", buildPackage(t, "bar", "1.0-1", "x86_64"))},
	}
	if err := svc.UploadFiles("myrepo", files); err != nil {
		t.Fatalf("UploadFiles: %v", err)
	}
}

func TestUploadFiles_RejectsSamePackageTwicePerArch(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	bin := mocks.NewMockBinaryRepository(ctrl)
	name := mocks.NewMockNameStore(ctrl)
	bin.EXPECT().VerifyPkgRepo("myrepo").Return(nil)
	bin.EXPECT().RemoteRepo("myrepo", "x86_64").Return(&repo.RemoteRepo{}, nil)

	svc := service.New(name, bin, nil, nil, baseConfig(false, ""))
	err := svc.UploadFiles("myrepo", []*domain.UploadFiles{
		{
			PkgFile: pkgStream(
				"foo-1.0-1-x86_64.pkg.tar.zst",
				buildPackage(t, "foo", "1.0-1", "x86_64"),
			),
		},
		{
			PkgFile: pkgStream(
				"foo-1.1-1-x86_64.pkg.tar.zst",
				buildPackage(t, "foo", "1.1-1", "x86_64"),
			),
		},
	})
	if !errors.Is(err, domain.ErrInvalidUpload) {
		t.Fatalf("same package twice = %v, want ErrInvalidUpload", err)
	}
}
