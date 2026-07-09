package service_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/Hayao0819/Kamisato/internal/errors"

	"go.uber.org/mock/gomock"

	"github.com/Hayao0819/Kamisato/ayato/repository"
	"github.com/Hayao0819/Kamisato/ayato/repository/blob"
	"github.com/Hayao0819/Kamisato/ayato/repository/blob/localfs"
	"github.com/Hayao0819/Kamisato/ayato/service"
	"github.com/Hayao0819/Kamisato/ayato/stream"
	"github.com/Hayao0819/Kamisato/ayato/test/mocks"
	"github.com/Hayao0819/Kamisato/internal/conf"
	pkgpkg "github.com/Hayao0819/Kamisato/pkg/pacman/pkg"
	"github.com/Hayao0819/Kamisato/pkg/pacman/repo"
	"github.com/Hayao0819/Kamisato/pkg/raiou"
)

type readSeekCloser struct{ *bytes.Reader }

func (readSeekCloser) Close() error { return nil }

func bufferToReadSeekCloser(buf *bytes.Buffer) readSeekCloser {
	return readSeekCloser{bytes.NewReader(buf.Bytes())}
}

func TestServiceRepoNames(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	bin := mocks.NewMockBinaryRepository(ctrl)
	bin.EXPECT().RepoNames().Return([]string{"core", "extra"}, nil)

	svc := service.New(mocks.NewMockNameStore(ctrl), bin, nil, nil, &conf.AyatoConfig{})
	got, err := svc.RepoNames()
	if err != nil {
		t.Fatalf("RepoNames failed: %v", err)
	}
	if len(got) != 2 || got[0] != "core" {
		t.Errorf("RepoNames = %v, want [core extra]", got)
	}
}

func TestServiceArches(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	bin := mocks.NewMockBinaryRepository(ctrl)
	bin.EXPECT().Arches("myrepo").Return([]string{"x86_64", "aarch64"}, nil)

	svc := service.New(mocks.NewMockNameStore(ctrl), bin, nil, nil, &conf.AyatoConfig{})
	got, err := svc.Arches("myrepo")
	if err != nil {
		t.Fatalf("Arches failed: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("Arches = %v, want 2 entries", got)
	}
}

func TestServiceGetFile(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	want := "package-bytes"
	fs := stream.NewFileStream("foo.pkg.tar.zst", "application/octet-stream",
		bufferToReadSeekCloser(bytes.NewBufferString(want)))

	bin := mocks.NewMockBinaryRepository(ctrl)
	bin.EXPECT().FetchFileWithMeta("myrepo", "x86_64", "foo.pkg.tar.zst").Return(fs, blob.FileMeta{ETag: `"etag1"`}, nil)

	svc := service.New(mocks.NewMockNameStore(ctrl), bin, nil, nil, &conf.AyatoConfig{})
	got, gotMeta, err := svc.GetFileWithMeta("myrepo", "x86_64", "foo.pkg.tar.zst")
	if err != nil {
		t.Fatalf("GetFileWithMeta failed: %v", err)
	}
	if gotMeta.ETag != `"etag1"` {
		t.Errorf("etag = %q, want %q", gotMeta.ETag, `"etag1"`)
	}
	buf := new(bytes.Buffer)
	if _, err := buf.ReadFrom(got); err != nil {
		t.Fatalf("read stream: %v", err)
	}
	if buf.String() != want {
		t.Errorf("GetFileWithMeta content = %q, want %q", buf.String(), want)
	}
}

func TestServiceSignedURL(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	bin := mocks.NewMockBinaryRepository(ctrl)
	bin.EXPECT().StoreFileWithSignedURL("r", "a", "n").Return("https://example.com/n", nil)

	svc := service.New(mocks.NewMockNameStore(ctrl), bin, nil, nil, &conf.AyatoConfig{})
	got, err := svc.SignedURL("r", "a", "n")
	if err != nil {
		t.Fatalf("SignedURL failed: %v", err)
	}
	if got != "https://example.com/n" {
		t.Errorf("SignedURL = %q, want %q", got, "https://example.com/n")
	}
}

func TestServiceRemovePkg(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	bin := mocks.NewMockBinaryRepository(ctrl)
	name := mocks.NewMockNameStore(ctrl)

	// ValidateRepoName -> RepoNames contains "myrepo" -> ok
	bin.EXPECT().RepoNames().Return([]string{"myrepo"}, nil)
	name.EXPECT().PackageFile("x86_64", "mypkg").Return("mypkg-1.0-1-x86_64.pkg.tar.zst", nil)
	// Concrete package scoped to its arch: de-registered, file deleted; Files lists no sig, so none removed.
	bin.EXPECT().RepoRemove("myrepo", "x86_64", "mypkg", false, gomock.Nil()).Return(nil)
	bin.EXPECT().DeleteFile("myrepo", "x86_64", "mypkg-1.0-1-x86_64.pkg.tar.zst").Return(nil)
	bin.EXPECT().Files("myrepo", "x86_64").Return([]string{"mypkg-1.0-1-x86_64.pkg.tar.zst", "myrepo.db"}, nil)
	name.EXPECT().DeletePackageFileEntry("x86_64", "mypkg").Return(nil)

	svc := service.New(name, bin, nil, nil, &conf.AyatoConfig{})
	if err := svc.RemovePkg("myrepo", "x86_64", "mypkg"); err != nil {
		t.Fatalf("RemovePkg failed: %v", err)
	}
}

func TestServiceRemovePkgAny(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	bin := mocks.NewMockBinaryRepository(ctrl)
	name := mocks.NewMockNameStore(ctrl)

	cfg := &conf.AyatoConfig{
		Repos: []conf.BinRepoConfig{{Name: "myrepo"}},
	}

	bin.EXPECT().RepoNames().Return([]string{"myrepo"}, nil) // ValidateRepoName
	bin.EXPECT().Arches("myrepo").Return([]string{"x86_64", "aarch64"}, nil).AnyTimes()
	name.EXPECT().PackageFile("any", "mypkg").Return("mypkg-1.0-1-any.pkg.tar.zst", nil)
	// arch="" (blinky route) on an any package: de-registered from every arch db; its file and sig live once under "any/" and go with the last arch.
	bin.EXPECT().RepoRemove("myrepo", "x86_64", "mypkg", false, gomock.Nil()).Return(nil)
	bin.EXPECT().RepoRemove("myrepo", "aarch64", "mypkg", false, gomock.Nil()).Return(nil)
	bin.EXPECT().DeleteFile("myrepo", "any", "mypkg-1.0-1-any.pkg.tar.zst").Return(nil)
	bin.EXPECT().Files("myrepo", "any").Return([]string{"mypkg-1.0-1-any.pkg.tar.zst", "mypkg-1.0-1-any.pkg.tar.zst.sig"}, nil)
	bin.EXPECT().DeleteFile("myrepo", "any", "mypkg-1.0-1-any.pkg.tar.zst.sig").Return(nil)
	name.EXPECT().DeletePackageFileEntry("any", "mypkg").Return(nil)

	svc := service.New(name, bin, nil, nil, cfg)
	if err := svc.RemovePkg("myrepo", "", "mypkg"); err != nil {
		t.Fatalf("RemovePkg(any, all arches) failed: %v", err)
	}
}

func TestServiceRemovePkgAnyOneArch(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	bin := mocks.NewMockBinaryRepository(ctrl)
	name := mocks.NewMockNameStore(ctrl)

	cfg := &conf.AyatoConfig{
		Repos: []conf.BinRepoConfig{{Name: "myrepo"}},
	}

	bin.EXPECT().RepoNames().Return([]string{"myrepo"}, nil) // ValidateRepoName
	bin.EXPECT().Arches("myrepo").Return([]string{"x86_64", "aarch64"}, nil).AnyTimes()
	// Scoped to x86_64; the any package's file is keyed under "any".
	name.EXPECT().PackageFile("x86_64", "mypkg").Return("", nil) // concrete miss
	name.EXPECT().PackageFile("any", "mypkg").Return("mypkg-1.0-1-any.pkg.tar.zst", nil)
	bin.EXPECT().RepoRemove("myrepo", "x86_64", "mypkg", false, gomock.Nil()).Return(nil)
	// aarch64 still lists it, so the shared any/ file is kept: no DeleteFile, no metadata deletion.
	stillThere := &repo.RemoteRepo{Pkgs: []*pkgpkg.BinaryPackage{
		pkgpkg.NewBinaryPackage("mypkg-1.0-1-any.pkg.tar.zst", &raiou.PKGINFO{PkgName: "mypkg", Arch: "any"}),
	}}
	bin.EXPECT().RemoteRepo("myrepo", "aarch64").Return(stillThere, nil)

	svc := service.New(name, bin, nil, nil, cfg)
	if err := svc.RemovePkg("myrepo", "x86_64", "mypkg"); err != nil {
		t.Fatalf("RemovePkg(any one arch) failed: %v", err)
	}
}

func TestServicePkgFilesError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	bin := mocks.NewMockBinaryRepository(ctrl)
	bin.EXPECT().PkgFiles("r", "a", "p").Return(nil, errors.New("boom"))

	svc := service.New(mocks.NewMockNameStore(ctrl), bin, nil, nil, &conf.AyatoConfig{})
	if _, err := svc.PkgFiles("r", "a", "p"); err == nil {
		t.Fatal("expected error from PkgFiles")
	}
}

func TestServiceLocalfsIntegration(t *testing.T) {
	repoRoot := t.TempDir()
	archDir := filepath.Join(repoRoot, "myrepo", "x86_64")
	if err := os.MkdirAll(archDir, 0o755); err != nil {
		t.Fatal(err)
	}
	const content = "dummy package payload"
	if err := os.WriteFile(filepath.Join(archDir, "foo.pkg.tar.zst"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := &conf.AyatoConfig{
		Repos: []conf.BinRepoConfig{{Name: "myrepo"}},
	}
	cfg.Store.StorageType = "localfs"
	cfg.Store.LocalRepoDir = repoRoot

	binRepo := repository.NewBinaryRepository(localfs.New(repoRoot, []string{"myrepo"}))
	svc := service.New(nil, binRepo, nil, nil, cfg)

	t.Run("RepoNames", func(t *testing.T) {
		names, err := svc.RepoNames()
		if err != nil {
			t.Fatalf("RepoNames failed: %v", err)
		}
		if len(names) != 1 || names[0] != "myrepo" {
			t.Errorf("RepoNames = %v, want [myrepo]", names)
		}
	})

	t.Run("Arches", func(t *testing.T) {
		arches, err := svc.Arches("myrepo")
		if err != nil {
			t.Fatalf("Arches failed: %v", err)
		}
		if len(arches) != 1 || arches[0] != "x86_64" {
			t.Errorf("Arches = %v, want [x86_64]", arches)
		}
	})

	t.Run("GetFile", func(t *testing.T) {
		f, _, err := svc.GetFileWithMeta("myrepo", "x86_64", "foo.pkg.tar.zst")
		if err != nil {
			t.Fatalf("GetFileWithMeta failed: %v", err)
		}
		defer f.Close()
		buf := new(bytes.Buffer)
		if _, err := buf.ReadFrom(f); err != nil {
			t.Fatalf("read file: %v", err)
		}
		if buf.String() != content {
			t.Errorf("GetFile content = %q, want %q", buf.String(), content)
		}
	})
}
