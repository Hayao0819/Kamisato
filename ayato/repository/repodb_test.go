package repository

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"path"
	"sort"
	"sync"
	"testing"

	"github.com/Hayao0819/Kamisato/ayato/stream"
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

func requireRepoAddTool(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("repo-add"); err != nil {
		t.Skip("repo-add not installed; skipping repo-db artifact test")
	}
}

// makePkg builds a minimal valid .pkg.tar.zst that repo-add accepts: a tar with
// a .PKGINFO at its root, zstd-compressed via bsdtar.
func makePkg(t *testing.T, dir, name, ver, arch string) string {
	t.Helper()
	if _, err := exec.LookPath("bsdtar"); err != nil {
		t.Skip("bsdtar not installed; skipping repo-add artifact test")
	}
	pkginfo := "pkgname = " + name + "\n" +
		"pkgver = " + ver + "\n" +
		"pkgdesc = test\n" +
		"arch = " + arch + "\n" +
		"size = 0\n"
	work := t.TempDir()
	if err := os.WriteFile(path.Join(work, ".PKGINFO"), []byte(pkginfo), 0o644); err != nil {
		t.Fatal(err)
	}
	out := path.Join(dir, name+"-"+ver+"-"+arch+".pkg.tar.zst")
	cmd := exec.Command("bsdtar", "-c", "--zstd", "-f", out, "-C", work, ".PKGINFO")
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("bsdtar: %v: %s", err, b)
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
// blob.Store, NOT just .db.tar.gz. This is the localfs/s3 unification.
func TestRepoDBArtifactSet(t *testing.T) {
	requireRepoAddTool(t)

	want := []string{"r.db", "r.db.tar.gz", "r.files", "r.files.tar.gz"}

	mem := newMemStore()
	r := &binaryRepository{Store: mem}

	if err := r.InitArch("r", "x86_64", false, nil); err != nil {
		t.Fatalf("InitArch: %v", err)
	}
	got := mem.names("r", "x86_64")
	assertSuperset(t, got, want, "InitArch")

	// RepoAdd a package; the artifact set (excluding the package file, which the
	// caller stores separately) must still be the full quartet.
	dir := t.TempDir()
	pkgPath := makePkg(t, dir, "foo", "1.0-1", "x86_64")
	pkg := openSeek(t, pkgPath)
	if err := r.RepoAdd("r", "x86_64", pkg, nil, false, nil); err != nil {
		t.Fatalf("RepoAdd: %v", err)
	}
	got = mem.names("r", "x86_64")
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
	got = mem.names("r", "x86_64")
	assertSuperset(t, got, want, "RepoRemove")
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
