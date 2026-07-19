package service_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/Hayao0819/Kamisato/internal/errors"
	"go.uber.org/mock/gomock"

	"github.com/Hayao0819/Kamisato/ayato/domain"
	"github.com/Hayao0819/Kamisato/ayato/platform"
	"github.com/Hayao0819/Kamisato/ayato/repository"
	"github.com/Hayao0819/Kamisato/ayato/repository/blob"
	"github.com/Hayao0819/Kamisato/ayato/repository/blob/localfs"
	"github.com/Hayao0819/Kamisato/ayato/service"
	"github.com/Hayao0819/Kamisato/ayato/test/mocks"
	"github.com/Hayao0819/Kamisato/internal/conf"
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

func TestValidateRepoNameUsesConfiguredCatalogAsAuthority(t *testing.T) {
	ctrl := gomock.NewController(t)
	bin := mocks.NewMockBinaryRepository(ctrl)
	svc := service.New(nil, bin, nil, nil, &conf.AyatoConfig{
		Repos: []conf.BinRepoConfig{{Name: "configured"}},
	})

	if err := svc.ValidateRepoName("leftover"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("ValidateRepoName(leftover) = %v, want ErrNotFound", err)
	}
	if err := svc.ValidateRepoName(".."); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("ValidateRepoName(..) = %v, want ErrInvalid", err)
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

	const want = "package-bytes"
	file := platform.NewFileStream(
		"foo.pkg.tar.zst",
		"application/octet-stream",
		bufferToReadSeekCloser(bytes.NewBufferString(want)),
	)
	bin := mocks.NewMockBinaryRepository(ctrl)
	bin.EXPECT().FetchFileWithMeta("myrepo", "x86_64", "foo.pkg.tar.zst").
		Return(file, blob.FileMeta{ETag: `"etag1"`}, nil)

	svc := service.New(mocks.NewMockNameStore(ctrl), bin, nil, nil, &conf.AyatoConfig{})
	got, metadata, err := svc.GetFileWithMeta("myrepo", "x86_64", "foo.pkg.tar.zst")
	if err != nil {
		t.Fatalf("GetFileWithMeta failed: %v", err)
	}
	if metadata.ETag != `"etag1"` {
		t.Errorf("etag = %q, want %q", metadata.ETag, `"etag1"`)
	}
	content := new(bytes.Buffer)
	if _, err := content.ReadFrom(got); err != nil {
		t.Fatalf("read stream: %v", err)
	}
	if content.String() != want {
		t.Errorf("content = %q, want %q", content.String(), want)
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

	cfg := &conf.AyatoConfig{Repos: []conf.BinRepoConfig{{Name: "myrepo"}}}
	cfg.Store.StorageType = "localfs"
	cfg.Store.LocalRepoDir = repoRoot
	binRepo := repository.NewBinaryRepository(localfs.New(repoRoot, []string{"myrepo"}))
	svc := service.New(nil, binRepo, nil, nil, cfg)

	names, err := svc.RepoNames()
	if err != nil || len(names) != 1 || names[0] != "myrepo" {
		t.Fatalf("RepoNames = %v, %v; want [myrepo]", names, err)
	}
	arches, err := svc.Arches("myrepo")
	if err != nil || len(arches) != 1 || arches[0] != "x86_64" {
		t.Fatalf("Arches = %v, %v; want [x86_64]", arches, err)
	}
	file, _, err := svc.GetFileWithMeta("myrepo", "x86_64", "foo.pkg.tar.zst")
	if err != nil {
		t.Fatalf("GetFileWithMeta: %v", err)
	}
	defer file.Close()
	got := new(bytes.Buffer)
	if _, err := got.ReadFrom(file); err != nil {
		t.Fatalf("read file: %v", err)
	}
	if got.String() != content {
		t.Errorf("content = %q, want %q", got.String(), content)
	}
}
