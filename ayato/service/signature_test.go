package service_test

import (
	"bytes"
	"fmt"
	"testing"

	"go.uber.org/mock/gomock"

	"github.com/Hayao0819/Kamisato/ayato/blob"
	"github.com/Hayao0819/Kamisato/ayato/service"
	"github.com/Hayao0819/Kamisato/ayato/test/mocks"
	pacmanpkg "github.com/Hayao0819/Kamisato/internal/pacman/pkg"
	"github.com/Hayao0819/Kamisato/internal/pacman/repo"
	"github.com/Hayao0819/Kamisato/pkg/raiou"
)

const sigTestFilename = "demo-1.0-1-x86_64.pkg.tar.zst"

func sigTestRemote() *repo.RemoteRepo {
	return &repo.RemoteRepo{Pkgs: []*pacmanpkg.BinaryPackage{
		pacmanpkg.NewBinaryPackage(sigTestFilename, &raiou.PKGINFO{
			PkgName: "demo",
			PkgBase: "demo",
			PkgVer:  "1.0-1",
			Arch:    "x86_64",
		}),
	}}
}

func fileFromBytes(name string, data []byte) blob.File {
	return blob.NewFileStream(
		name,
		"application/octet-stream",
		bufferToReadSeekCloser(bytes.NewBuffer(data)),
	)
}

func TestPkgSignature_Unsigned(t *testing.T) {
	ctrl := gomock.NewController(t)
	bin := mocks.NewMockBinaryRepository(ctrl)
	bin.EXPECT().RemoteRepo("myrepo", "x86_64").Return(sigTestRemote(), nil)
	bin.EXPECT().
		FetchFile("myrepo", "x86_64", sigTestFilename+".sig").
		Return(nil, blob.ErrNotFound)

	svc := service.New(mocks.NewMockNameStore(ctrl), bin, nil, nil, service.Settings{})
	got, err := svc.PkgSignature("myrepo", "x86_64", "demo")
	if err != nil {
		t.Fatalf("PkgSignature: %v", err)
	}
	if got.Present {
		t.Fatalf("signature = %+v, want absent", got)
	}
}

func TestPkgSignature_Present(t *testing.T) {
	ctrl := gomock.NewController(t)
	signer := newSigner(t)
	sig := detachSignBytes(t, signer, []byte("package bytes"))

	bin := mocks.NewMockBinaryRepository(ctrl)
	bin.EXPECT().RemoteRepo("myrepo", "x86_64").Return(sigTestRemote(), nil)
	bin.EXPECT().
		FetchFile("myrepo", "x86_64", sigTestFilename+".sig").
		Return(fileFromBytes(sigTestFilename+".sig", sig), nil)

	svc := service.New(mocks.NewMockNameStore(ctrl), bin, nil, nil, service.Settings{})
	got, err := svc.PkgSignature("myrepo", "x86_64", "demo")
	if err != nil {
		t.Fatalf("PkgSignature: %v", err)
	}
	wantFpr := fmt.Sprintf("%X", signer.PrimaryKey.Fingerprint)
	if !got.Present || got.Filename != sigTestFilename+".sig" || got.Fingerprint != wantFpr {
		t.Fatalf("signature = %+v, want present with fingerprint %s", got, wantFpr)
	}
	if got.KeyID == "" || got.CreatedAt == 0 || got.PubKeyAlgo != "EdDSA" {
		t.Errorf("signature metadata incomplete: %+v", got)
	}
}
