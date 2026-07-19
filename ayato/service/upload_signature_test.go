package service_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/Hayao0819/Kamisato/internal/errors"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/ProtonMail/go-crypto/openpgp/packet"
	"go.uber.org/mock/gomock"

	"github.com/Hayao0819/Kamisato/ayato/domain"
	"github.com/Hayao0819/Kamisato/ayato/repository"
	"github.com/Hayao0819/Kamisato/ayato/service"
	"github.com/Hayao0819/Kamisato/ayato/stream"
	"github.com/Hayao0819/Kamisato/ayato/test/mocks"
	"github.com/Hayao0819/Kamisato/pkg/pacman/repo"
)

func newSigner(t *testing.T) *openpgp.Entity {
	t.Helper()
	entity, err := openpgp.NewEntity(
		"signer",
		"test",
		"signer@example.com",
		&packet.Config{Algorithm: packet.PubKeyAlgoEdDSA},
	)
	if err != nil {
		t.Fatalf("NewEntity: %v", err)
	}
	return entity
}

func writeKeyring(t *testing.T, entity *openpgp.Entity) string {
	t.Helper()
	var keyring bytes.Buffer
	if err := entity.Serialize(&keyring); err != nil {
		t.Fatalf("serialize: %v", err)
	}
	path := filepath.Join(t.TempDir(), "keyring.gpg")
	if err := os.WriteFile(path, keyring.Bytes(), 0o600); err != nil {
		t.Fatalf("write keyring: %v", err)
	}
	return path
}

func detachSignBytes(t *testing.T, signer *openpgp.Entity, payload []byte) []byte {
	t.Helper()
	var signature bytes.Buffer
	if err := openpgp.DetachSign(
		&signature,
		signer,
		bytes.NewReader(payload),
		&packet.Config{Algorithm: packet.PubKeyAlgoEdDSA},
	); err != nil {
		t.Fatalf("DetachSign: %v", err)
	}
	return signature.Bytes()
}

func TestUploadFile_RequireSignNoSig(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	keyring := writeKeyring(t, newSigner(t))
	bin := mocks.NewMockBinaryRepository(ctrl)
	name := mocks.NewMockNameStore(ctrl)
	bin.EXPECT().VerifyPkgRepo("myrepo").Return(nil)
	bin.EXPECT().RemoteRepo("myrepo", "x86_64").Return(&repo.RemoteRepo{}, nil).AnyTimes()

	svc := service.New(name, bin, nil, nil, baseConfig(true, keyring))
	files := &domain.UploadFiles{
		PkgFile: pkgStream(uploadName, buildPackage(t, "foo", "1.0-1", "x86_64")),
	}
	err := svc.UploadFile("myrepo", files)
	if !errors.Is(err, domain.ErrInvalidUpload) {
		t.Fatalf("missing signature = %v, want ErrInvalidUpload", err)
	}
}

func TestUploadFile_BadSigRejected(t *testing.T) {
	for _, requireSign := range []bool{false, true} {
		t.Run("requireSign="+boolString(requireSign), func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			trustedSigner := newSigner(t)
			payload := buildPackage(t, "foo", "1.0-1", "x86_64")
			badSignature := detachSignBytes(t, newSigner(t), payload)

			bin := mocks.NewMockBinaryRepository(ctrl)
			name := mocks.NewMockNameStore(ctrl)
			bin.EXPECT().VerifyPkgRepo("myrepo").Return(nil)
			bin.EXPECT().RemoteRepo("myrepo", "x86_64").Return(&repo.RemoteRepo{}, nil).AnyTimes()

			svc := service.New(name, bin, nil, nil, baseConfig(requireSign, writeKeyring(t, trustedSigner)))
			err := svc.UploadFile("myrepo", &domain.UploadFiles{
				PkgFile: pkgStream(uploadName, payload),
				SigFile: pkgStream(uploadName+".sig", badSignature),
			})
			if !errors.Is(err, domain.ErrInvalidUpload) {
				t.Fatalf("untrusted signature = %v, want ErrInvalidUpload", err)
			}
		})
	}
}

func TestUploadFile_GoodSigStoresPackageAndSignature(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	signer := newSigner(t)
	payload := buildPackage(t, "foo", "1.0-1", "x86_64")
	signature := detachSignBytes(t, signer, payload)
	bin := mocks.NewMockBinaryRepository(ctrl)
	name := mocks.NewMockNameStore(ctrl)
	bin.EXPECT().VerifyPkgRepo("myrepo").Return(nil)
	bin.EXPECT().RemoteRepo("myrepo", "x86_64").Return(&repo.RemoteRepo{}, nil).AnyTimes()

	var stored []string
	bin.EXPECT().StoreFileImmutable("myrepo", "x86_64", gomock.Any()).DoAndReturn(
		func(_, _ string, file stream.SeekFile) (bool, error) {
			stored = append(stored, file.FileName())
			return true, nil
		},
	).Times(2)
	bin.EXPECT().RepoAddBatch("myrepo", "x86_64", gomock.Any(), false, gomock.Nil()).Return(nil)
	name.EXPECT().StorePackageFiles("myrepo", []repository.PackageFileEntry{
		{Arch: "x86_64", Name: "foo", FileName: uploadName},
	}).Return(nil)

	svc := service.New(name, bin, nil, nil, baseConfig(false, writeKeyring(t, signer)))
	err := svc.UploadFile("myrepo", &domain.UploadFiles{
		PkgFile: pkgStream(uploadName, payload),
		SigFile: pkgStream(uploadName+".sig", signature),
	})
	if err != nil {
		t.Fatalf("UploadFile: %v", err)
	}
	if len(stored) != 2 || stored[0] != uploadName || stored[1] != uploadName+".sig" {
		t.Errorf("stored files = %v, want [%s %s]", stored, uploadName, uploadName+".sig")
	}
}

func boolString(value bool) string {
	if value {
		return "true"
	}
	return "false"
}
