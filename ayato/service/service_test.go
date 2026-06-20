package service_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/Hayao0819/Kamisato/ayato/repository"
	"github.com/Hayao0819/Kamisato/ayato/repository/store/localfs"
	"github.com/Hayao0819/Kamisato/ayato/service"
	"github.com/Hayao0819/Kamisato/ayato/stream"
	"github.com/Hayao0819/Kamisato/ayato/test/mocks"
	"github.com/Hayao0819/Kamisato/internal/conf"
	"github.com/Hayao0819/Kamisato/internal/utils"
	"go.uber.org/mock/gomock"
)

// --- mock-based unit tests ---

func TestServiceRepoNames(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	bin := mocks.NewMockBinaryRepository(ctrl)
	bin.EXPECT().RepoNames().Return([]string{"core", "extra"}, nil)

	svc := service.New(mocks.NewMockNameStore(ctrl), bin, &conf.AyatoConfig{})
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

	svc := service.New(mocks.NewMockNameStore(ctrl), bin, &conf.AyatoConfig{})
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
		utils.BufferToReadSeekCloser(bytes.NewBufferString(want)))

	bin := mocks.NewMockBinaryRepository(ctrl)
	bin.EXPECT().FetchFile("myrepo", "x86_64", "foo.pkg.tar.zst").Return(fs, nil)

	svc := service.New(mocks.NewMockNameStore(ctrl), bin, &conf.AyatoConfig{})
	got, err := svc.GetFile("myrepo", "x86_64", "foo.pkg.tar.zst")
	if err != nil {
		t.Fatalf("GetFile failed: %v", err)
	}
	buf := new(bytes.Buffer)
	if _, err := buf.ReadFrom(got); err != nil {
		t.Fatalf("read stream: %v", err)
	}
	if buf.String() != want {
		t.Errorf("GetFile content = %q, want %q", buf.String(), want)
	}
}

func TestServiceSignedURL(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	bin := mocks.NewMockBinaryRepository(ctrl)
	bin.EXPECT().StoreFileWithSignedURL("r", "a", "n").Return("https://example.com/n", nil)

	svc := service.New(mocks.NewMockNameStore(ctrl), bin, &conf.AyatoConfig{})
	got, err := svc.SignedURL("r", "a", "n")
	if err != nil {
		t.Fatalf("SignedURL failed: %v", err)
	}
	if got != "https://example.com/n" {
		t.Errorf("SignedURL = %q", got)
	}
}

func TestServiceRemovePkg(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	bin := mocks.NewMockBinaryRepository(ctrl)
	name := mocks.NewMockNameStore(ctrl)

	// ValidateRepoName -> RepoNames contains "myrepo" -> ok
	bin.EXPECT().RepoNames().Return([]string{"myrepo"}, nil)
	name.EXPECT().PackageFile("mypkg").Return("mypkg-1.0-1-x86_64.pkg.tar.zst", nil)
	bin.EXPECT().DeleteFile("myrepo", "x86_64", "mypkg-1.0-1-x86_64.pkg.tar.zst").Return(nil)
	name.EXPECT().DeletePackageFileEntry("mypkg").Return(nil)

	svc := service.New(name, bin, &conf.AyatoConfig{})
	if err := svc.RemovePkg("myrepo", "x86_64", "mypkg"); err != nil {
		t.Fatalf("RemovePkg failed: %v", err)
	}
}

func TestServicePkgFilesError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	bin := mocks.NewMockBinaryRepository(ctrl)
	bin.EXPECT().PkgFiles("r", "a", "p").Return(nil, errors.New("boom"))

	svc := service.New(mocks.NewMockNameStore(ctrl), bin, &conf.AyatoConfig{})
	if _, err := svc.PkgFiles("r", "a", "p"); err == nil {
		t.Fatal("expected error from PkgFiles")
	}
}

// --- integration test: service -> repository -> localfs (real filesystem) ---

func TestServiceLocalfsIntegration(t *testing.T) {
	repoRoot := t.TempDir()
	// repoRoot/myrepo/x86_64/foo.pkg.tar.zst
	archDir := filepath.Join(repoRoot, "myrepo", "x86_64")
	if err := os.MkdirAll(archDir, 0o755); err != nil {
		t.Fatal(err)
	}
	const content = "dummy package payload"
	if err := os.WriteFile(filepath.Join(archDir, "foo.pkg.tar.zst"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := &conf.AyatoConfig{
		Repos: []conf.BinRepoConfig{{Name: "myrepo", Arches: []string{"x86_64"}}},
	}
	cfg.Store.StorageType = "localfs"
	cfg.Store.LocalRepoDir = repoRoot

	binRepo := repository.NewBinaryRepository(localfs.New(cfg), cfg)
	svc := service.New(nil, binRepo, cfg)

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
		f, err := svc.GetFile("myrepo", "x86_64", "foo.pkg.tar.zst")
		if err != nil {
			t.Fatalf("GetFile failed: %v", err)
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
