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

// pkgWithFields builds a foo-1.0-1 .pkg.tar.zst whose .PKGINFO carries the given
// pkgname plus extra raw lines (provides/replaces/group entries).
func pkgWithFields(t *testing.T, pkgname, extra string) []byte {
	t.Helper()
	body := "pkgname = " + pkgname + "\npkgver = 1.0-1\narch = x86_64\n" + extra
	var tarBuf bytes.Buffer
	tw := tar.NewWriter(&tarBuf)
	if err := tw.WriteHeader(&tar.Header{Name: ".PKGINFO", Mode: 0o644, Size: int64(len(body))}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write([]byte(body)); err != nil {
		t.Fatal(err)
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

func protectedConfig(names ...string) *conf.AyatoConfig {
	return &conf.AyatoConfig{
		ProtectedNames: names,
		Repos:          []conf.BinRepoConfig{{Name: "myrepo"}},
	}
}

func TestUploadFile_ProtectedNameCollision(t *testing.T) {
	cases := []struct {
		name    string
		pkgname string
		extra   string
	}{
		{"pkgname", "pacman", ""},
		{"provides", "evil", "provides = glibc=2.39\n"},
		{"replaces", "evil", "replaces = pacman\n"},
		{"groups", "evil", "group = base\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			bin := mocks.NewMockBinaryRepository(ctrl)
			name := mocks.NewMockNameStore(ctrl)
			bin.EXPECT().VerifyPkgRepo("myrepo").Return(nil)
			// No StoreFile / RepoAddBatch: the guard rejects before anything is stored.

			svc := service.New(name, bin, nil, nil, protectedConfig("pacman", "glibc", "base"))
			files := &domain.UploadFiles{PkgFile: pkgStream(uploadName, pkgWithFields(t, tc.pkgname, tc.extra))}
			err := svc.UploadFile("myrepo", files)
			if !errors.Is(err, domain.ErrConflict) {
				t.Fatalf("a protected-name collision must be ErrConflict, got %v", err)
			}
		})
	}
}

func TestUploadFile_ProtectedNamesAllowNonCollision(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	bin := mocks.NewMockBinaryRepository(ctrl)
	name := mocks.NewMockNameStore(ctrl)
	bin.EXPECT().VerifyPkgRepo("myrepo").Return(nil)
	bin.EXPECT().RemoteRepo("myrepo", "x86_64").Return(&repo.RemoteRepo{}, nil).AnyTimes()
	bin.EXPECT().StoreFile("myrepo", "x86_64", gomock.Any()).Return(nil)
	bin.EXPECT().RepoAddBatch("myrepo", "x86_64", gomock.Any(), false, gomock.Nil()).Return(nil)
	name.EXPECT().StorePackageFile("myrepo", "x86_64", "foo", uploadName).Return(nil)

	svc := service.New(name, bin, nil, nil, protectedConfig("pacman", "glibc"))
	// foo provides a lookalike-but-distinct name; nothing collides with the list.
	files := &domain.UploadFiles{PkgFile: pkgStream(uploadName, pkgWithFields(t, "foo", "provides = libfoo.so=1-64\n"))}
	if err := svc.UploadFile("myrepo", files); err != nil {
		t.Fatalf("a non-colliding package must be accepted: %v", err)
	}
}
