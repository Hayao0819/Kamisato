package service_test

import (
	"archive/tar"
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/Hayao0819/Kamisato/internal/errors"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/ProtonMail/go-crypto/openpgp/packet"
	"github.com/klauspost/compress/zstd"
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

// remoteWith builds a one-package RemoteRepo for the version-gate tests.
func remoteWith(pkgname, version string) *repo.RemoteRepo {
	return &repo.RemoteRepo{Pkgs: []*pkgpkg.BinaryPackage{
		pkgpkg.NewBinaryPackage(pkgname+"-"+version+"-x86_64.pkg.tar.zst",
			&raiou.PKGINFO{PkgName: pkgname, Arch: "x86_64", PkgVer: version}),
	}}
}

const pkginfoBody = "pkgname = foo\n" +
	"pkgver = 1.0-1\n" +
	"arch = x86_64\n" +
	"xdata = pkgtype=pkg\n"

// buildPkgArchive returns a .pkg.tar.zst with a minimal .PKGINFO so
// ReadBinaryPackage can parse name/ver/arch.
func buildPkgArchive(t *testing.T) []byte {
	t.Helper()
	var tarBuf bytes.Buffer
	tw := tar.NewWriter(&tarBuf)
	hdr := &tar.Header{Name: ".PKGINFO", Mode: 0o644, Size: int64(len(pkginfoBody))}
	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatalf("tar header: %v", err)
	}
	if _, err := tw.Write([]byte(pkginfoBody)); err != nil {
		t.Fatalf("tar write: %v", err)
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("tar close: %v", err)
	}

	var zBuf bytes.Buffer
	zw, err := zstd.NewWriter(&zBuf)
	if err != nil {
		t.Fatalf("zstd writer: %v", err)
	}
	if _, err := zw.Write(tarBuf.Bytes()); err != nil {
		t.Fatalf("zstd write: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("zstd close: %v", err)
	}
	return zBuf.Bytes()
}

func newSigner(t *testing.T) *openpgp.Entity {
	t.Helper()
	e, err := openpgp.NewEntity("signer", "test", "signer@example.com", &packet.Config{Algorithm: packet.PubKeyAlgoEdDSA})
	if err != nil {
		t.Fatalf("NewEntity: %v", err)
	}
	return e
}

func writeKeyring(t *testing.T, e *openpgp.Entity) string {
	t.Helper()
	var buf bytes.Buffer
	if err := e.Serialize(&buf); err != nil {
		t.Fatalf("serialize: %v", err)
	}
	p := filepath.Join(t.TempDir(), "keyring.gpg")
	if err := os.WriteFile(p, buf.Bytes(), 0o600); err != nil {
		t.Fatalf("write keyring: %v", err)
	}
	return p
}

func detachSignBytes(t *testing.T, signer *openpgp.Entity, payload []byte) []byte {
	t.Helper()
	var sig bytes.Buffer
	if err := openpgp.DetachSign(&sig, signer, bytes.NewReader(payload), &packet.Config{Algorithm: packet.PubKeyAlgoEdDSA}); err != nil {
		t.Fatalf("DetachSign: %v", err)
	}
	return sig.Bytes()
}

func pkgStream(name string, data []byte) *stream.FileStream {
	return stream.NewFileStream(name, "application/octet-stream", bufferToReadSeekCloser(bytes.NewBuffer(data)))
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

func TestUploadFile_RequireSignNoSig(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	signer := newSigner(t)
	keyring := writeKeyring(t, signer)

	bin := mocks.NewMockBinaryRepository(ctrl)
	name := mocks.NewMockNameStore(ctrl)
	bin.EXPECT().VerifyPkgRepo("myrepo").Return(nil)
	bin.EXPECT().RemoteRepo("myrepo", "x86_64").Return(&repo.RemoteRepo{}, nil).AnyTimes()
	// No StoreFile / RepoAdd / StorePackageFile must be called.

	svc := service.New(name, bin, nil, nil, baseConfig(true, keyring))
	files := &domain.UploadFiles{PkgFile: pkgStream(uploadName, buildPkgArchive(t))}
	err := svc.UploadFile("myrepo", files)
	if err == nil {
		t.Fatal("expected error when RequireSign and no signature, got nil")
	}
	if !errors.Is(err, domain.ErrInvalidUpload) {
		t.Fatalf("missing signature must be a client error (ErrInvalidUpload), got %v", err)
	}
}

func TestUploadFile_BadSigRejected(t *testing.T) {
	for _, requireSign := range []bool{false, true} {
		t.Run("requireSign="+boolStr(requireSign), func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			signer := newSigner(t)
			other := newSigner(t)
			keyring := writeKeyring(t, signer)
			payload := buildPkgArchive(t)
			// signature made by a key NOT in the keyring -> untrusted/unknown.
			badSig := detachSignBytes(t, other, payload)

			bin := mocks.NewMockBinaryRepository(ctrl)
			name := mocks.NewMockNameStore(ctrl)
			bin.EXPECT().VerifyPkgRepo("myrepo").Return(nil)
			bin.EXPECT().RemoteRepo("myrepo", "x86_64").Return(&repo.RemoteRepo{}, nil).AnyTimes()
			// No StoreFile / RepoAdd allowed.

			svc := service.New(name, bin, nil, nil, baseConfig(requireSign, keyring))
			files := &domain.UploadFiles{
				PkgFile: pkgStream(uploadName, payload),
				SigFile: pkgStream(uploadName+".sig", badSig),
			}
			err := svc.UploadFile("myrepo", files)
			if err == nil {
				t.Fatal("expected error for untrusted signature, got nil")
			}
			if !errors.Is(err, domain.ErrInvalidUpload) {
				t.Fatalf("untrusted signature must be a client error (ErrInvalidUpload), got %v", err)
			}
		})
	}
}

func TestUploadFile_GoodSigStoresTwice(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	signer := newSigner(t)
	keyring := writeKeyring(t, signer)
	payload := buildPkgArchive(t)
	sig := detachSignBytes(t, signer, payload)

	bin := mocks.NewMockBinaryRepository(ctrl)
	name := mocks.NewMockNameStore(ctrl)
	bin.EXPECT().VerifyPkgRepo("myrepo").Return(nil)
	bin.EXPECT().RemoteRepo("myrepo", "x86_64").Return(&repo.RemoteRepo{}, nil).AnyTimes()

	var stored []string
	bin.EXPECT().StoreFileImmutable("myrepo", "x86_64", gomock.Any()).DoAndReturn(
		func(_, _ string, f stream.SeekFile) (bool, error) {
			stored = append(stored, f.FileName())
			return true, nil
		}).Times(2)
	bin.EXPECT().RepoAddBatch("myrepo", "x86_64", gomock.Any(), false, gomock.Nil()).Return(nil)
	name.EXPECT().StorePackageFiles("myrepo", []repository.PackageFileEntry{{Arch: "x86_64", Name: "foo", FileName: uploadName}}).Return(nil)

	svc := service.New(name, bin, nil, nil, baseConfig(false, keyring))
	files := &domain.UploadFiles{
		PkgFile: pkgStream(uploadName, payload),
		SigFile: pkgStream(uploadName+".sig", sig),
	}
	if err := svc.UploadFile("myrepo", files); err != nil {
		t.Fatalf("UploadFile: %v", err)
	}
	if len(stored) != 2 || stored[0] != uploadName || stored[1] != uploadName+".sig" {
		t.Errorf("StoreFile order = %v, want [%s %s]", stored, uploadName, uploadName+".sig")
	}
}

func TestUploadFile_DowngradeRejected(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	bin := mocks.NewMockBinaryRepository(ctrl)
	name := mocks.NewMockNameStore(ctrl)
	bin.EXPECT().VerifyPkgRepo("myrepo").Return(nil)
	// foo 2.0-1 is already published; the upload is the older 1.0-1.
	bin.EXPECT().RemoteRepo("myrepo", "x86_64").Return(remoteWith("foo", "2.0-1"), nil)
	// No StoreFile / RepoAddBatch / StorePackageFile must be called.

	svc := service.New(name, bin, nil, nil, baseConfig(false, ""))
	files := &domain.UploadFiles{PkgFile: pkgStream(uploadName, buildPkgArchive(t))}
	err := svc.UploadFile("myrepo", files)
	if err == nil {
		t.Fatal("expected a downgrade to be rejected, got nil")
	}
	if !errors.Is(err, domain.ErrInvalidUpload) {
		t.Fatalf("downgrade must be a client error (ErrInvalidUpload), got %v", err)
	}
}

func TestUploadFile_DuplicateRejected(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	bin := mocks.NewMockBinaryRepository(ctrl)
	name := mocks.NewMockNameStore(ctrl)
	bin.EXPECT().VerifyPkgRepo("myrepo").Return(nil)
	// The same version 1.0-1 is already published.
	bin.EXPECT().RemoteRepo("myrepo", "x86_64").Return(remoteWith("foo", "1.0-1"), nil)

	svc := service.New(name, bin, nil, nil, baseConfig(false, ""))
	files := &domain.UploadFiles{PkgFile: pkgStream(uploadName, buildPkgArchive(t))}
	err := svc.UploadFile("myrepo", files)
	if err == nil {
		t.Fatal("expected a duplicate version to be rejected, got nil")
	}
	if !errors.Is(err, domain.ErrInvalidUpload) {
		t.Fatalf("duplicate must be a client error (ErrInvalidUpload), got %v", err)
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
		PkgFile: pkgStream(uploadName, buildRollbackPkg(t, "evil", "1.0-1", "x86_64")),
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
	bin.EXPECT().VerifyPkgRepo("myrepo").Return(nil)
	// The published 0.9-1 is older than the uploaded 1.0-1, so it is accepted.
	bin.EXPECT().RemoteRepo("myrepo", "x86_64").Return(remoteWith("foo", "0.9-1"), nil)
	bin.EXPECT().FetchFile("myrepo", "x86_64", "foo-0.9-1-x86_64.pkg.tar.zst").Return(pkgStream("foo-0.9-1-x86_64.pkg.tar.zst", buildPkgArchive(t)), nil)
	bin.EXPECT().FetchFile("myrepo", "x86_64", "foo-0.9-1-x86_64.pkg.tar.zst.sig").Return(nil, blob.ErrNotFound)
	bin.EXPECT().StoreFileImmutable("myrepo", "x86_64", gomock.Any()).Return(true, nil)
	bin.EXPECT().RepoAddBatch("myrepo", "x86_64", gomock.Any(), false, gomock.Nil()).Return(nil)
	name.EXPECT().StorePackageFiles("myrepo", []repository.PackageFileEntry{{Arch: "x86_64", Name: "foo", FileName: uploadName}}).Return(nil)

	svc := service.New(name, bin, nil, nil, baseConfig(false, ""))
	files := &domain.UploadFiles{PkgFile: pkgStream(uploadName, buildPkgArchive(t))}
	if err := svc.UploadFile("myrepo", files); err != nil {
		t.Fatalf("an upgrade should be accepted: %v", err)
	}
}

func TestUploadFile_NameStoreFailureRestoresPreviousVersion(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	bin := mocks.NewMockBinaryRepository(ctrl)
	name := mocks.NewMockNameStore(ctrl)
	oldName := "foo-0.9-1-x86_64.pkg.tar.zst"
	bin.EXPECT().VerifyPkgRepo("myrepo").Return(nil)
	bin.EXPECT().RemoteRepo("myrepo", "x86_64").Return(remoteWith("foo", "0.9-1"), nil)
	bin.EXPECT().FetchFile("myrepo", "x86_64", oldName).Return(pkgStream(oldName, buildPkgArchive(t)), nil)
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
		}).Times(2)

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
		}).Times(2)
	name.EXPECT().StorePackageFiles("myrepo", []repository.PackageFileEntry{{Arch: "x86_64", Name: "foo", FileName: uploadName}}).Return(errors.New("name store unavailable"))
	name.EXPECT().PackageFile("myrepo", "x86_64", "foo").Return(uploadName, nil)
	name.EXPECT().DeletePackageFileEntry("myrepo", "x86_64", "foo").Return(nil)
	name.EXPECT().StorePackageFiles("myrepo", []repository.PackageFileEntry{{Arch: "x86_64", Name: "foo", FileName: oldName}}).Return(nil)

	svc := service.New(name, bin, nil, nil, baseConfig(false, ""))
	files := &domain.UploadFiles{PkgFile: pkgStream(uploadName, buildPkgArchive(t))}
	if err := svc.UploadFile("myrepo", files); err == nil {
		t.Fatal("expected name-store failure")
	}
	if repoAdds != 2 {
		t.Fatalf("RepoAddBatch calls = %d, want publish + restore", repoAdds)
	}
	if storeCalls != 2 {
		t.Fatalf("StoreFileImmutable calls = %d, want publish + old-object renewal", storeCalls)
	}
}

func TestUploadFile_DBFailureLeavesImmutableObjectsForReconcile(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	signer := newSigner(t)
	keyring := writeKeyring(t, signer)
	payload := buildPkgArchive(t)
	sig := detachSignBytes(t, signer, payload)

	bin := mocks.NewMockBinaryRepository(ctrl)
	name := mocks.NewMockNameStore(ctrl)
	bin.EXPECT().VerifyPkgRepo("myrepo").Return(nil)
	bin.EXPECT().RemoteRepo("myrepo", "x86_64").Return(&repo.RemoteRepo{}, nil).AnyTimes()
	bin.EXPECT().StoreFileImmutable("myrepo", "x86_64", gomock.Any()).Return(true, nil).Times(2)
	bin.EXPECT().RepoAddBatch("myrepo", "x86_64", gomock.Any(), false, gomock.Nil()).Return(errors.New("boom"))

	svc := service.New(name, bin, nil, nil, baseConfig(false, keyring))
	files := &domain.UploadFiles{
		PkgFile: pkgStream(uploadName, payload),
		SigFile: pkgStream(uploadName+".sig", sig),
	}
	if err := svc.UploadFile("myrepo", files); err == nil {
		t.Fatal("expected error from failing RepoAdd, got nil")
	}
}

func TestUploadFile_AtomicPackageStoreFailureReturns(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	bin := mocks.NewMockBinaryRepository(ctrl)
	name := mocks.NewMockNameStore(ctrl)
	bin.EXPECT().VerifyPkgRepo("myrepo").Return(nil)
	bin.EXPECT().RemoteRepo("myrepo", "x86_64").Return(&repo.RemoteRepo{}, nil)
	bin.EXPECT().StoreFileImmutable("myrepo", "x86_64", gomock.Any()).Return(false, errors.New("atomic object write failed"))
	svc := service.New(name, bin, nil, nil, baseConfig(false, ""))
	if err := svc.UploadFile("myrepo", &domain.UploadFiles{PkgFile: pkgStream(uploadName, buildPkgArchive(t))}); err == nil {
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
	bin.EXPECT().StoreFileImmutable("myrepo", "x86_64", gomock.Any()).Return(false, repository.ErrImmutableObjectConflict)
	svc := service.New(name, bin, nil, nil, baseConfig(false, ""))
	err := svc.UploadFile("myrepo", &domain.UploadFiles{PkgFile: pkgStream(uploadName, buildPkgArchive(t))})
	if !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("immutable object conflict = %v, want ErrConflict", err)
	}
}

func TestUploadFile_AtomicSignatureStoreFailureReturns(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	signer := newSigner(t)
	payload := buildPkgArchive(t)
	sig := detachSignBytes(t, signer, payload)
	bin := mocks.NewMockBinaryRepository(ctrl)
	name := mocks.NewMockNameStore(ctrl)
	bin.EXPECT().VerifyPkgRepo("myrepo").Return(nil)
	bin.EXPECT().RemoteRepo("myrepo", "x86_64").Return(&repo.RemoteRepo{}, nil)
	gomock.InOrder(
		bin.EXPECT().StoreFileImmutable("myrepo", "x86_64", gomock.Any()).Return(true, nil),
		bin.EXPECT().StoreFileImmutable("myrepo", "x86_64", gomock.Any()).Return(false, errors.New("atomic signature write failed")),
	)
	cfg := baseConfig(false, writeKeyring(t, signer))
	svc := service.New(name, bin, nil, nil, cfg)
	err := svc.UploadFile("myrepo", &domain.UploadFiles{
		PkgFile: pkgStream(uploadName, payload),
		SigFile: pkgStream(uploadName+".sig", sig),
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
	bin.EXPECT().RepoAddBatch("myrepo", "x86_64", gomock.Any(), false, gomock.Nil()).Return(&repository.CanonicalCommitError{Err: errors.New("files artifact failed")})
	bin.EXPECT().RepoRemoveIfMatch("myrepo", "x86_64", "foo", "1.0-1", uploadName, false, gomock.Nil()).Return(&repository.CanonicalCommitError{Err: errors.New("rollback files artifact failed")})
	bin.EXPECT().ReconcileDB("myrepo", "x86_64", false, gomock.Nil()).Return(nil)
	svc := service.New(name, bin, nil, nil, baseConfig(false, ""))
	if err := svc.UploadFile("myrepo", &domain.UploadFiles{PkgFile: pkgStream(uploadName, buildPkgArchive(t))}); err == nil {
		t.Fatal("derived artifact failure was reported as success")
	}
}

func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

// buildNamedPkg is buildPkgArchive with a chosen pkgname, for multi-package batch
// tests.
func buildNamedPkg(t *testing.T, name string) []byte {
	return buildRollbackPkg(t, name, "1.0-1", "x86_64")
}

func buildRollbackPkg(t *testing.T, name, version, arch string) []byte {
	t.Helper()
	body := "pkgname = " + name + "\npkgver = " + version + "\narch = " + arch + "\nxdata = pkgtype=pkg\n"
	var tarBuf bytes.Buffer
	tw := tar.NewWriter(&tarBuf)
	if err := tw.WriteHeader(&tar.Header{Name: ".PKGINFO", Mode: 0o644, Size: int64(len(body))}); err != nil {
		t.Fatalf("tar header: %v", err)
	}
	if _, err := tw.Write([]byte(body)); err != nil {
		t.Fatalf("tar write: %v", err)
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("tar close: %v", err)
	}
	var zBuf bytes.Buffer
	zw, err := zstd.NewWriter(&zBuf)
	if err != nil {
		t.Fatalf("zstd writer: %v", err)
	}
	if _, err := zw.Write(tarBuf.Bytes()); err != nil {
		t.Fatalf("zstd write: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("zstd close: %v", err)
	}
	return zBuf.Bytes()
}

func TestUploadFiles_SecondArchFailureRestoresFirstArch(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	bin := mocks.NewMockBinaryRepository(ctrl)
	name := mocks.NewMockNameStore(ctrl)
	cfg := &conf.AyatoConfig{Repos: []conf.BinRepoConfig{{Name: "myrepo", Arches: []string{"x86_64", "aarch64"}}}}
	oldName := "foo-0.9-1-any.pkg.tar.zst"
	oldRepo := &repo.RemoteRepo{Pkgs: []*pkgpkg.BinaryPackage{
		pkgpkg.NewBinaryPackage(oldName, &raiou.PKGINFO{PkgName: "foo", PkgVer: "0.9-1", Arch: "any"}),
	}}
	bin.EXPECT().VerifyPkgRepo("myrepo").Return(nil)
	bin.EXPECT().Arches("myrepo").Return([]string{"x86_64", "aarch64"}, nil).AnyTimes()
	bin.EXPECT().RemoteRepo("myrepo", "x86_64").Return(oldRepo, nil)
	bin.EXPECT().RemoteRepo("myrepo", "aarch64").Return(oldRepo, nil)
	bin.EXPECT().FetchFile("myrepo", "any", oldName).Return(pkgStream(oldName, buildRollbackPkg(t, "foo", "0.9-1", "any")), nil)
	bin.EXPECT().FetchFile("myrepo", "any", oldName+".sig").Return(nil, blob.ErrNotFound)
	storeCalls := 0
	bin.EXPECT().StoreFileImmutable("myrepo", "any", gomock.Any()).DoAndReturn(
		func(_ string, _ string, file stream.SeekFile) (bool, error) {
			storeCalls++
			want := "foo-1.0-1-any.pkg.tar.zst"
			if storeCalls == 2 {
				want = oldName
			}
			if file.FileName() != want {
				t.Fatalf("StoreFileImmutable call %d stored %q, want %q", storeCalls, file.FileName(), want)
			}
			return true, nil
		}).Times(2)

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
		}).Times(2)
	bin.EXPECT().RepoAddBatch("myrepo", "aarch64", gomock.Any(), false, gomock.Nil()).Return(errors.New("aarch64 commit failed"))

	svc := service.New(name, bin, nil, nil, cfg)
	files := []*domain.UploadFiles{{PkgFile: pkgStream("foo-1.0-1-any.pkg.tar.zst", buildRollbackPkg(t, "foo", "1.0-1", "any"))}}
	if err := svc.UploadFiles("myrepo", files); err == nil {
		t.Fatal("expected second-arch commit failure")
	}
	if x86Calls != 2 {
		t.Fatalf("x86_64 commits = %d, want publish + restore", x86Calls)
	}
	if storeCalls != 2 {
		t.Fatalf("StoreFileImmutable calls = %d, want publish + old-object renewal", storeCalls)
	}
}

// countingSignerRepo records how often the signer keyring is rebuilt.
type countingSignerRepo struct{ listCalls int }

func (r *countingSignerRepo) AddSigner(string, []byte) error { return nil }
func (r *countingSignerRepo) DeleteSigner(string) error      { return nil }
func (r *countingSignerRepo) ListSigners() ([][]byte, error) { r.listCalls++; return nil, nil }

// TestUploadFiles_KeyringBuiltOncePerBatch proves the verification keyring is
// built a single time for a multi-package signed batch rather than once per file:
// the signer lookup (KV-backed in production) must not fan out per package.
func TestUploadFiles_KeyringBuiltOncePerBatch(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	signer := newSigner(t)
	keyring := writeKeyring(t, signer)
	fooPayload := buildNamedPkg(t, "foo")
	barPayload := buildNamedPkg(t, "bar")
	fooSig := detachSignBytes(t, signer, fooPayload)
	barSig := detachSignBytes(t, signer, barPayload)

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

	sr := &countingSignerRepo{}
	svc := service.New(name, bin, nil, sr, baseConfig(false, keyring))
	files := []*domain.UploadFiles{
		{PkgFile: pkgStream("foo-1.0-1-x86_64.pkg.tar.zst", fooPayload), SigFile: pkgStream("foo-1.0-1-x86_64.pkg.tar.zst.sig", fooSig)},
		{PkgFile: pkgStream("bar-1.0-1-x86_64.pkg.tar.zst", barPayload), SigFile: pkgStream("bar-1.0-1-x86_64.pkg.tar.zst.sig", barSig)},
	}
	if err := svc.UploadFiles("myrepo", files); err != nil {
		t.Fatalf("UploadFiles: %v", err)
	}
	if sr.listCalls != 1 {
		t.Fatalf("signer keyring rebuilt %d times, want exactly 1 per batch", sr.listCalls)
	}
}

// TestUploadFiles_BatchOneRepoAddPerArch proves a multi-package publish enters the
// (repo, arch) database with a SINGLE RepoAddBatch (default Times(1)) rather than
// one RepoAdd per package — the atomic-batch payoff.
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
		{PkgFile: pkgStream("foo-1.0-1-x86_64.pkg.tar.zst", buildNamedPkg(t, "foo"))},
		{PkgFile: pkgStream("bar-1.0-1-x86_64.pkg.tar.zst", buildNamedPkg(t, "bar"))},
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
				buildRollbackPkg(t, "foo", "1.0-1", "x86_64"),
			),
		},
		{
			PkgFile: pkgStream(
				"foo-1.1-1-x86_64.pkg.tar.zst",
				buildRollbackPkg(t, "foo", "1.1-1", "x86_64"),
			),
		},
	})
	if !errors.Is(err, domain.ErrInvalidUpload) {
		t.Fatalf("same package twice = %v, want ErrInvalidUpload", err)
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
	bin.EXPECT().RepoAddBatch("myrepo", "x86_64", gomock.Any(), false, gomock.Nil()).Return(
		&repository.CanonicalCommitError{Err: repository.ErrPackageChanged},
	)
	bin.EXPECT().RepoRemoveIfMatch("myrepo", "x86_64", "foo", "1.0-1", "foo-1.0-1-x86_64.pkg.tar.zst", false, gomock.Nil()).Return(repository.ErrPackageChanged)
	bin.EXPECT().RepoRemoveIfMatch("myrepo", "x86_64", "bar", "1.0-1", "bar-1.0-1-x86_64.pkg.tar.zst", false, gomock.Nil()).Return(nil)
	bin.EXPECT().ReconcileDB("myrepo", "x86_64", false, gomock.Nil()).Return(nil)

	svc := service.New(name, bin, nil, nil, baseConfig(false, ""))
	err := svc.UploadFiles("myrepo", []*domain.UploadFiles{
		{PkgFile: pkgStream("foo-1.0-1-x86_64.pkg.tar.zst", buildNamedPkg(t, "foo"))},
		{PkgFile: pkgStream("bar-1.0-1-x86_64.pkg.tar.zst", buildNamedPkg(t, "bar"))},
	})
	if !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("superseded batch = %v, want ErrConflict", err)
	}
}
