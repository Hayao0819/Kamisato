package repository

import (
	"archive/tar"
	"bytes"
	"io"
	"os"
	"os/exec"
	"path"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/Hayao0819/Kamisato/ayato/stream"
	"github.com/Hayao0819/Kamisato/pkg/pacman/repo"
	"github.com/klauspost/compress/zstd"
)

// memStore is an in-memory blob.Store. It is backend-agnostic, so asserting the
// artifact set it receives from binaryRepository's RepoAdd/RepoRemove/InitArch
// proves localfs and s3 (the two real blob.Store implementations) are handed the
// identical set: repodb only ever talks to the blob.Store contract.
type memStore struct {
	mu    sync.Mutex
	files map[string][]byte // "repo/arch/name" -> bytes
}

func newMemStore() *memStore { return &memStore{files: map[string][]byte{}} }

func (m *memStore) keyOf(repo, arch, name string) string { return repo + "/" + arch + "/" + name }

func (m *memStore) StoreFile(repo, arch string, file stream.SeekFile) error {
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return err
	}
	b, err := io.ReadAll(file)
	if err != nil {
		return err
	}
	m.mu.Lock()
	m.files[m.keyOf(repo, arch, path.Base(file.FileName()))] = b
	m.mu.Unlock()
	return nil
}

func (m *memStore) StoreFileWithSignedURL(string, string, string) (string, error) { return "", nil }
func (m *memStore) DeleteFile(repo, arch, name string) error {
	m.mu.Lock()
	delete(m.files, m.keyOf(repo, arch, name))
	m.mu.Unlock()
	return nil
}

func (m *memStore) FetchFile(repo, arch, name string) (stream.File, error) {
	m.mu.Lock()
	b, ok := m.files[m.keyOf(repo, arch, name)]
	m.mu.Unlock()
	if !ok {
		return nil, os.ErrNotExist
	}
	return stream.NewFileStream(name, "application/octet-stream", nopSeekCloser{bytes.NewReader(b)}), nil
}

func (m *memStore) RepoNames() ([]string, error)           { return nil, nil }
func (m *memStore) Files(string, string) ([]string, error) { return nil, nil }
func (m *memStore) Arches(string) ([]string, error)        { return nil, nil }

func (m *memStore) names(repo, arch string) []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []string
	prefix := repo + "/" + arch + "/"
	for k := range m.files {
		if len(k) > len(prefix) && k[:len(prefix)] == prefix {
			out = append(out, k[len(prefix):])
		}
	}
	sort.Strings(out)
	return out
}

type nopSeekCloser struct{ *bytes.Reader }

func (nopSeekCloser) Close() error { return nil }

// makePkg builds a minimal valid .pkg.tar.zst in pure Go: a zstd-compressed tar
// with a .PKGINFO at its root. It needs no external tooling, so the repo-DB
// tests run on any distribution — the point of the native writer.
func makePkg(t *testing.T, dir, name, ver, arch string) string {
	t.Helper()
	pkginfo := "pkgname = " + name + "\n" +
		"pkgver = " + ver + "\n" +
		"pkgdesc = test\n" +
		"arch = " + arch + "\n" +
		"size = 0\n"
	out := path.Join(dir, name+"-"+ver+"-"+arch+".pkg.tar.zst")
	f, err := os.Create(out)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	zw, err := zstd.NewWriter(f)
	if err != nil {
		t.Fatal(err)
	}
	tw := tar.NewWriter(zw)
	if err := tw.WriteHeader(&tar.Header{
		Name: ".PKGINFO", Mode: 0o644, Size: int64(len(pkginfo)), Typeflag: tar.TypeReg,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write([]byte(pkginfo)); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return out
}

func openSeek(t *testing.T, p string) stream.SeekFile {
	t.Helper()
	f, err := stream.OpenFileWithType(p)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { f.Close() })
	return f
}

// TestRepoDBArtifactSet locks the unified artifact set: InitArch and RepoAdd must
// write the full quartet (.db, .db.tar.gz, .files, .files.tar.gz) through the
// blob.Store, NOT just .db.tar.gz. This is the localfs/s3 unification. It runs
// the orchestration against BOTH backends — the default Go-native writer (no
// repo-add needed) and the repo-add CLI (skipped when absent) — proving the
// fetch -> tool -> store cycle is backend-agnostic.
func TestRepoDBArtifactSet(t *testing.T) {
	backends := []struct {
		name         string
		tool         repoDBTool
		needsRepoAdd bool
	}{
		{"native", nil, false},
		{"cli", repo.CLITool{}, true},
	}

	for _, be := range backends {
		t.Run(be.name, func(t *testing.T) {
			if be.needsRepoAdd {
				if _, err := exec.LookPath("repo-add"); err != nil {
					t.Skip("repo-add not installed; skipping CLI backend")
				}
			}

			want := []string{"r.db", "r.db.tar.gz", "r.files", "r.files.tar.gz"}
			mem := newMemStore()
			r := &binaryRepository{Store: mem, tool: be.tool}

			if err := r.InitArch("r", "x86_64", false, nil); err != nil {
				t.Fatalf("InitArch: %v", err)
			}
			assertSuperset(t, mem.names("r", "x86_64"), want, "InitArch")

			// RepoAdd a package; the artifact set (excluding the package file,
			// which the caller stores separately) must still be the full quartet.
			dir := t.TempDir()
			pkgPath := makePkg(t, dir, "foo", "1.0-1", "x86_64")
			pkgStream := openSeek(t, pkgPath)
			if err := r.RepoAdd("r", "x86_64", pkgStream, nil, false, nil); err != nil {
				t.Fatalf("RepoAdd: %v", err)
			}
			got := mem.names("r", "x86_64")
			assertSuperset(t, got, want, "RepoAdd")
			if contains(got, path.Base(pkgPath)) {
				t.Errorf("RepoAdd stored the package file %q through the DB path; the caller's StoreFile owns it", path.Base(pkgPath))
			}

			// The package is now registered: RemoteRepo must see it.
			rr, err := r.RemoteRepo("r", "x86_64")
			if err != nil {
				t.Fatalf("RemoteRepo: %v", err)
			}
			if len(rr.Pkgs) != 1 {
				t.Fatalf("expected 1 package after RepoAdd, got %d", len(rr.Pkgs))
			}

			// RepoRemove drops it again, keeping the same artifact set.
			if err := r.RepoRemove("r", "x86_64", "foo", false, nil); err != nil {
				t.Fatalf("RepoRemove: %v", err)
			}
			assertSuperset(t, mem.names("r", "x86_64"), want, "RepoRemove")
		})
	}
}

func assertSuperset(t *testing.T, got, want []string, ctx string) {
	t.Helper()
	for _, w := range want {
		if !contains(got, w) {
			t.Errorf("%s: artifact %q missing; got %v", ctx, w, got)
		}
	}
}

func contains(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

// fakeTool stands in for the repo-add/repo-remove CLI: it just writes the
// canonical DB quartet next to the db path, so the binaryRepository
// orchestration (fetch -> tool -> store) can be exercised without the repo-add
// binary. This is the payoff of the repoDBTool port.
type fakeTool struct{}

func (fakeTool) RepoAdd(dbPath, _ string, _ bool, _ *string) error { return writeFakeQuartet(dbPath) }
func (fakeTool) RepoRemove(dbPath, _ string, _ bool, _ *string) error {
	return writeFakeQuartet(dbPath)
}

func writeFakeQuartet(dbPath string) error {
	dir := path.Dir(dbPath)
	base := strings.TrimSuffix(path.Base(dbPath), ".db.tar.gz")
	for _, n := range []string{base + ".db", base + ".db.tar.gz", base + ".files", base + ".files.tar.gz"} {
		if err := os.WriteFile(path.Join(dir, n), []byte("db"), 0o644); err != nil {
			return err
		}
	}
	return nil
}

// TestRepoDBToolPort verifies, with an injected fake tool, that the
// binaryRepository orchestration stores the full unified artifact set and never
// stores the package file through the DB path. It needs no repo-add binary, so
// it runs everywhere.
func TestRepoDBToolPort(t *testing.T) {
	want := []string{"r.db", "r.db.tar.gz", "r.files", "r.files.tar.gz"}
	mem := newMemStore()
	r := &binaryRepository{Store: mem, tool: fakeTool{}}

	if err := r.InitArch("r", "x86_64", false, nil); err != nil {
		t.Fatalf("InitArch: %v", err)
	}
	assertSuperset(t, mem.names("r", "x86_64"), want, "InitArch")

	pkg := stream.NewFileStream("foo-1.0-1-x86_64.pkg.tar.zst", "application/octet-stream",
		nopSeekCloser{bytes.NewReader([]byte("pkg"))})
	if err := r.RepoAdd("r", "x86_64", pkg, nil, false, nil); err != nil {
		t.Fatalf("RepoAdd: %v", err)
	}
	got := mem.names("r", "x86_64")
	assertSuperset(t, got, want, "RepoAdd")
	if contains(got, "foo-1.0-1-x86_64.pkg.tar.zst") {
		t.Errorf("RepoAdd stored the package file through the DB path; the caller's StoreFile owns it")
	}

	if err := r.RepoRemove("r", "x86_64", "foo", false, nil); err != nil {
		t.Fatalf("RepoRemove: %v", err)
	}
	assertSuperset(t, mem.names("r", "x86_64"), want, "RepoRemove")
}
