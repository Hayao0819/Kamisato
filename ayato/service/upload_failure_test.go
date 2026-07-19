package service_test

import (
	"testing"

	"github.com/Hayao0819/Kamisato/internal/errors"
	"go.uber.org/mock/gomock"

	"github.com/Hayao0819/Kamisato/ayato/domain"
	"github.com/Hayao0819/Kamisato/ayato/repository"
	"github.com/Hayao0819/Kamisato/ayato/repository/blob"
	"github.com/Hayao0819/Kamisato/ayato/service"
	"github.com/Hayao0819/Kamisato/ayato/stream"
	"github.com/Hayao0819/Kamisato/ayato/test/mocks"
	"github.com/Hayao0819/Kamisato/pkg/pacman/repo"
)

func TestUploadFile_NameStoreFailureRestoresPreviousVersion(t *testing.T) {
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

	storeCalls := 0
	bin.EXPECT().StoreFileImmutable("myrepo", "x86_64", gomock.Any()).DoAndReturn(
		func(_ string, _ string, file stream.SeekFile) (bool, error) {
			storeCalls++
			wantName := uploadName
			if storeCalls == 2 {
				wantName = oldName
			}
			if file.FileName() != wantName {
				t.Fatalf("StoreFileImmutable call %d stored %q, want %q", storeCalls, file.FileName(), wantName)
			}
			return true, nil
		},
	).Times(2)

	repoAdds := 0
	bin.EXPECT().RepoAddBatch("myrepo", "x86_64", gomock.Any(), false, gomock.Nil()).DoAndReturn(
		func(_ string, _ string, items []repository.RepoAddItem, _ bool, _ *string) error {
			repoAdds++
			wantName := uploadName
			if repoAdds == 2 {
				wantName = oldName
			}
			if len(items) != 1 || items[0].Pkg.FileName() != wantName {
				t.Fatalf("RepoAddBatch call %d restored %v, want %s", repoAdds, items, wantName)
			}
			return nil
		},
	).Times(2)
	name.EXPECT().StorePackageFiles("myrepo", []repository.PackageFileEntry{
		{Arch: "x86_64", Name: "foo", FileName: uploadName},
	}).Return(errors.New("name store unavailable"))
	name.EXPECT().PackageFile("myrepo", "x86_64", "foo").Return(uploadName, nil)
	name.EXPECT().DeletePackageFileEntry("myrepo", "x86_64", "foo").Return(nil)
	name.EXPECT().StorePackageFiles("myrepo", []repository.PackageFileEntry{
		{Arch: "x86_64", Name: "foo", FileName: oldName},
	}).Return(nil)

	svc := service.New(name, bin, nil, nil, baseConfig(false, ""))
	files := &domain.UploadFiles{
		PkgFile: pkgStream(uploadName, buildPackage(t, "foo", "1.0-1", "x86_64")),
	}
	if err := svc.UploadFile("myrepo", files); err == nil {
		t.Fatal("expected name-store failure")
	}
	if repoAdds != 2 || storeCalls != 2 {
		t.Fatalf("restore calls = repo:%d store:%d, want 2 each", repoAdds, storeCalls)
	}
}

func TestUploadFile_DBFailureLeavesImmutableObjectsForReconcile(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	signer := newSigner(t)
	payload := buildPackage(t, "foo", "1.0-1", "x86_64")
	bin := mocks.NewMockBinaryRepository(ctrl)
	name := mocks.NewMockNameStore(ctrl)
	bin.EXPECT().VerifyPkgRepo("myrepo").Return(nil)
	bin.EXPECT().RemoteRepo("myrepo", "x86_64").Return(&repo.RemoteRepo{}, nil).AnyTimes()
	bin.EXPECT().StoreFileImmutable("myrepo", "x86_64", gomock.Any()).Return(true, nil).Times(2)
	bin.EXPECT().RepoAddBatch("myrepo", "x86_64", gomock.Any(), false, gomock.Nil()).
		Return(errors.New("boom"))

	svc := service.New(name, bin, nil, nil, baseConfig(false, writeKeyring(t, signer)))
	err := svc.UploadFile("myrepo", &domain.UploadFiles{
		PkgFile: pkgStream(uploadName, payload),
		SigFile: pkgStream(uploadName+".sig", detachSignBytes(t, signer, payload)),
	})
	if err == nil {
		t.Fatal("expected error from failing RepoAdd")
	}
}

func TestUploadFile_AtomicPackageStoreFailureReturns(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	bin := mocks.NewMockBinaryRepository(ctrl)
	name := mocks.NewMockNameStore(ctrl)
	bin.EXPECT().VerifyPkgRepo("myrepo").Return(nil)
	bin.EXPECT().RemoteRepo("myrepo", "x86_64").Return(&repo.RemoteRepo{}, nil)
	bin.EXPECT().StoreFileImmutable("myrepo", "x86_64", gomock.Any()).
		Return(false, errors.New("atomic object write failed"))

	svc := service.New(name, bin, nil, nil, baseConfig(false, ""))
	err := svc.UploadFile("myrepo", &domain.UploadFiles{
		PkgFile: pkgStream(uploadName, buildPackage(t, "foo", "1.0-1", "x86_64")),
	})
	if err == nil {
		t.Fatal("package store failure was reported as success")
	}
}

func TestUploadFile_ImmutableContentConflictStopsBeforeDBMutation(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	bin := mocks.NewMockBinaryRepository(ctrl)
	name := mocks.NewMockNameStore(ctrl)
	bin.EXPECT().VerifyPkgRepo("myrepo").Return(nil)
	bin.EXPECT().RemoteRepo("myrepo", "x86_64").Return(&repo.RemoteRepo{}, nil)
	bin.EXPECT().StoreFileImmutable("myrepo", "x86_64", gomock.Any()).
		Return(false, repository.ErrImmutableObjectConflict)

	svc := service.New(name, bin, nil, nil, baseConfig(false, ""))
	err := svc.UploadFile("myrepo", &domain.UploadFiles{
		PkgFile: pkgStream(uploadName, buildPackage(t, "foo", "1.0-1", "x86_64")),
	})
	if !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("immutable object conflict = %v, want ErrConflict", err)
	}
}

func TestUploadFile_AtomicSignatureStoreFailureReturns(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	signer := newSigner(t)
	payload := buildPackage(t, "foo", "1.0-1", "x86_64")
	bin := mocks.NewMockBinaryRepository(ctrl)
	name := mocks.NewMockNameStore(ctrl)
	bin.EXPECT().VerifyPkgRepo("myrepo").Return(nil)
	bin.EXPECT().RemoteRepo("myrepo", "x86_64").Return(&repo.RemoteRepo{}, nil)
	gomock.InOrder(
		bin.EXPECT().StoreFileImmutable("myrepo", "x86_64", gomock.Any()).Return(true, nil),
		bin.EXPECT().StoreFileImmutable("myrepo", "x86_64", gomock.Any()).
			Return(false, errors.New("atomic signature write failed")),
	)

	svc := service.New(name, bin, nil, nil, baseConfig(false, writeKeyring(t, signer)))
	err := svc.UploadFile("myrepo", &domain.UploadFiles{
		PkgFile: pkgStream(uploadName, payload),
		SigFile: pkgStream(uploadName+".sig", detachSignBytes(t, signer, payload)),
	})
	if err == nil {
		t.Fatal("signature store failure was reported as success")
	}
}

func TestUploadFile_DerivedArtifactFailureCompensatesCanonicalCommit(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	bin := mocks.NewMockBinaryRepository(ctrl)
	name := mocks.NewMockNameStore(ctrl)
	bin.EXPECT().VerifyPkgRepo("myrepo").Return(nil)
	bin.EXPECT().RemoteRepo("myrepo", "x86_64").Return(&repo.RemoteRepo{}, nil)
	bin.EXPECT().StoreFileImmutable("myrepo", "x86_64", gomock.Any()).Return(true, nil)
	bin.EXPECT().RepoAddBatch("myrepo", "x86_64", gomock.Any(), false, gomock.Nil()).
		Return(&repository.CanonicalCommitError{Err: errors.New("files artifact failed")})
	bin.EXPECT().RepoRemoveIfMatch(
		"myrepo", "x86_64", "foo", "1.0-1", uploadName, false, gomock.Nil(),
	).Return(&repository.CanonicalCommitError{Err: errors.New("rollback files artifact failed")})
	bin.EXPECT().ReconcileDB("myrepo", "x86_64", false, gomock.Nil()).Return(nil)

	svc := service.New(name, bin, nil, nil, baseConfig(false, ""))
	err := svc.UploadFile("myrepo", &domain.UploadFiles{
		PkgFile: pkgStream(uploadName, buildPackage(t, "foo", "1.0-1", "x86_64")),
	})
	if err == nil {
		t.Fatal("derived artifact failure was reported as success")
	}
}
