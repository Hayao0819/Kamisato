package service_test

import (
	"archive/tar"
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/ProtonMail/go-crypto/openpgp/packet"
	"github.com/klauspost/compress/zstd"
	"go.uber.org/mock/gomock"

	"github.com/Hayao0819/Kamisato/ayato/domain"
	"github.com/Hayao0819/Kamisato/ayato/service"
	"github.com/Hayao0819/Kamisato/ayato/stream"
	"github.com/Hayao0819/Kamisato/ayato/test/mocks"
	"github.com/Hayao0819/Kamisato/internal/conf"
	"github.com/Hayao0819/Kamisato/internal/utils"
)

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
	return stream.NewFileStream(name, "application/octet-stream", utils.BufferToReadSeekCloser(bytes.NewBuffer(data)))
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
	// No StoreFile / RepoAdd / StorePackageFile must be called.

	svc := service.New(name, bin, nil, baseConfig(true, keyring))
	files := &domain.UploadFiles{PkgFile: pkgStream(uploadName, buildPkgArchive(t))}
	if err := svc.UploadFile("myrepo", files); err == nil {
		t.Fatal("expected error when RequireSign and no signature, got nil")
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
			// No StoreFile / RepoAdd allowed.

			svc := service.New(name, bin, nil, baseConfig(requireSign, keyring))
			files := &domain.UploadFiles{
				PkgFile: pkgStream(uploadName, payload),
				SigFile: pkgStream(uploadName+".sig", badSig),
			}
			if err := svc.UploadFile("myrepo", files); err == nil {
				t.Fatal("expected error for untrusted signature, got nil")
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

	var stored []string
	bin.EXPECT().StoreFile("myrepo", "x86_64", gomock.Any()).DoAndReturn(
		func(_, _ string, f stream.SeekFile) error {
			stored = append(stored, f.FileName())
			return nil
		}).Times(2)
	bin.EXPECT().RepoAdd("myrepo", "x86_64", gomock.Any(), gomock.Nil(), false, gomock.Nil()).Return(nil)
	name.EXPECT().StorePackageFile("x86_64", "foo", uploadName).Return(nil)

	svc := service.New(name, bin, nil, baseConfig(false, keyring))
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

func TestUploadFile_CleanupRemovesBoth(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	signer := newSigner(t)
	keyring := writeKeyring(t, signer)
	payload := buildPkgArchive(t)
	sig := detachSignBytes(t, signer, payload)

	bin := mocks.NewMockBinaryRepository(ctrl)
	name := mocks.NewMockNameStore(ctrl)
	bin.EXPECT().VerifyPkgRepo("myrepo").Return(nil)
	// both pkg and sig get stored.
	bin.EXPECT().StoreFile("myrepo", "x86_64", gomock.Any()).Return(nil).Times(2)
	// RepoAdd fails, triggering cleanup of both stored files.
	bin.EXPECT().RepoAdd("myrepo", "x86_64", gomock.Any(), gomock.Nil(), false, gomock.Nil()).Return(errors.New("boom"))

	var deleted []string
	bin.EXPECT().DeleteFile("myrepo", "x86_64", gomock.Any()).DoAndReturn(
		func(_, _, f string) error {
			deleted = append(deleted, f)
			return nil
		}).Times(2)

	svc := service.New(name, bin, nil, baseConfig(false, keyring))
	files := &domain.UploadFiles{
		PkgFile: pkgStream(uploadName, payload),
		SigFile: pkgStream(uploadName+".sig", sig),
	}
	if err := svc.UploadFile("myrepo", files); err == nil {
		t.Fatal("expected error from failing RepoAdd, got nil")
	}
	if len(deleted) != 2 {
		t.Fatalf("DeleteFile calls = %v, want pkg + sig", deleted)
	}
	wantPkg, wantSig := false, false
	for _, d := range deleted {
		if d == uploadName {
			wantPkg = true
		}
		if d == uploadName+".sig" {
			wantSig = true
		}
	}
	if !wantPkg || !wantSig {
		t.Errorf("cleanup deleted = %v, want both %s and its .sig", deleted, uploadName)
	}
}

func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}
