package repository

import (
	"archive/tar"
	"bytes"
	"io"
	"os"
	"os/exec"
	"path"
	"testing"

	"github.com/klauspost/compress/zstd"

	"github.com/Hayao0819/Kamisato/ayato/stream"
	"github.com/Hayao0819/Kamisato/pkg/pacman/repo"
)

func makePkg(t *testing.T, dir, name, version, arch string) string {
	t.Helper()
	pkginfo := "pkgname = " + name + "\n" +
		"pkgver = " + version + "\n" +
		"pkgdesc = test\n" +
		"arch = " + arch + "\n" +
		"size = 0\n"
	output := path.Join(dir, name+"-"+version+"-"+arch+".pkg.tar.zst")
	file, err := os.Create(output)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	zstdWriter, err := zstd.NewWriter(file)
	if err != nil {
		t.Fatal(err)
	}
	tarWriter := tar.NewWriter(zstdWriter)
	header := &tar.Header{
		Name: ".PKGINFO", Mode: 0o644,
		Size: int64(len(pkginfo)), Typeflag: tar.TypeReg,
	}
	if err := tarWriter.WriteHeader(header); err != nil {
		t.Fatal(err)
	}
	if _, err := tarWriter.Write([]byte(pkginfo)); err != nil {
		t.Fatal(err)
	}
	if err := tarWriter.Close(); err != nil {
		t.Fatal(err)
	}
	if err := zstdWriter.Close(); err != nil {
		t.Fatal(err)
	}
	return output
}

func openSeek(t *testing.T, path string) stream.SeekFile {
	t.Helper()
	file, err := stream.OpenFileWithType(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = file.Close() })
	return file
}

func TestRepoDBArtifactSet(t *testing.T) {
	backends := []struct {
		name         string
		tool         repoDBTool
		needsRepoAdd bool
	}{
		{name: "native"},
		{name: "cli", tool: repo.CLITool{}, needsRepoAdd: true},
	}

	for _, backend := range backends {
		t.Run(backend.name, func(t *testing.T) {
			if backend.needsRepoAdd {
				if _, err := exec.LookPath("repo-add"); err != nil {
					t.Skip("repo-add not installed; skipping CLI backend")
				}
			}

			want := []string{"r.db.tar.gz", "r.files.tar.gz"}
			mem := newMemStore()
			repository := &binaryRepository{Store: mem, tool: backend.tool}
			if err := repository.InitArch("r", "x86_64", false, nil); err != nil {
				t.Fatalf("InitArch: %v", err)
			}
			assertSuperset(t, mem.names("r", "x86_64"), want, "InitArch")
			assertAliases(t, repository, mem, "InitArch")

			pkgPath := makePkg(t, t.TempDir(), "foo", "1.0-1", "x86_64")
			if err := repository.RepoAdd(
				"r", "x86_64", openSeek(t, pkgPath), nil, false, nil,
			); err != nil {
				t.Fatalf("RepoAdd: %v", err)
			}
			got := mem.names("r", "x86_64")
			assertSuperset(t, got, want, "RepoAdd")
			assertAliases(t, repository, mem, "RepoAdd")
			if contains(got, path.Base(pkgPath)) {
				t.Errorf("RepoAdd stored package %q through the DB path", path.Base(pkgPath))
			}

			remote, err := repository.RemoteRepo("r", "x86_64")
			if err != nil || len(remote.Pkgs) != 1 {
				t.Fatalf("RemoteRepo = %v, %v; want one package", remote, err)
			}
			if err := repository.RepoRemove("r", "x86_64", "foo", false, nil); err != nil {
				t.Fatalf("RepoRemove: %v", err)
			}
			assertSuperset(t, mem.names("r", "x86_64"), want, "RepoRemove")
			assertAliases(t, repository, mem, "RepoRemove")
		})
	}
}

func assertAliases(
	t *testing.T,
	repository *binaryRepository,
	mem *memStore,
	context string,
) {
	t.Helper()
	stored := mem.names("r", "x86_64")
	for _, bare := range []string{"r.db", "r.files"} {
		if contains(stored, bare) {
			t.Errorf("%s: %q was stored instead of served as an alias", context, bare)
		}
	}
	for bare, archive := range map[string]string{
		"r.db": "r.db.tar.gz", "r.files": "r.files.tar.gz",
	} {
		alias, err := repository.FetchFile("r", "x86_64", bare)
		if err != nil {
			t.Fatalf("%s: FetchFile(%q): %v", context, bare, err)
		}
		got, readErr := io.ReadAll(alias)
		_ = alias.Close()
		if readErr != nil {
			t.Fatalf("%s: read alias %q: %v", context, bare, readErr)
		}
		storedArchive, err := mem.FetchFile("r", "x86_64", archive)
		if err != nil {
			t.Fatalf("%s: FetchFile(%q): %v", context, archive, err)
		}
		want, _ := io.ReadAll(storedArchive)
		_ = storedArchive.Close()
		if !bytes.Equal(got, want) {
			t.Errorf("%s: alias %q does not match %q", context, bare, archive)
		}
	}
}

func assertSuperset(t *testing.T, got, want []string, context string) {
	t.Helper()
	for _, name := range want {
		if !contains(got, name) {
			t.Errorf("%s: artifact %q missing; got %v", context, name, got)
		}
	}
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
