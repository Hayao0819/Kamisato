package repository

import (
	"archive/tar"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/Hayao0819/Kamisato/ayato/repository/blob"
	"github.com/Hayao0819/Kamisato/ayato/stream"
	"github.com/Hayao0819/Kamisato/pkg/pacman/repo"
	"github.com/klauspost/compress/zstd"
)

// memStore is an in-memory blob.Store. It is backend-agnostic, so asserting the
// artifact set it receives from binaryRepository's RepoAdd/RepoRemove/InitArch
// proves localfs and s3 (the two real blob.Store implementations) are handed the
// identical set: repodb only ever talks to the blob.Store contract.
type memStore struct {
	mu       sync.Mutex
	files    map[string][]byte // "repo/arch/name" -> bytes
	versions map[string]string // "repo/arch/name" -> etag
	nextVer  int
	// onFetch, when set, fires once inside FetchFileWithETag (after the etag is
	// captured) to model a concurrent writer committing between this writer's read
	// and its compare-and-swap. One-shot.
	onFetch func(name string)
	// fetchErr, when set, is returned by FetchFileWithETag to model a transient
	// backend error (which must NOT be mistaken for absence).
	fetchErr error
}

func newMemStore() *memStore {
	return &memStore{files: map[string][]byte{}, versions: map[string]string{}}
}

func (m *memStore) keyOf(repo, arch, name string) string { return repo + "/" + arch + "/" + name }

// put records bytes and bumps the version; the caller holds mu.
func (m *memStore) put(repo, arch, name string, b []byte) {
	k := m.keyOf(repo, arch, name)
	m.files[k] = b
	m.nextVer++
	m.versions[k] = fmt.Sprintf("v%d", m.nextVer)
}

func readAllSeek(file stream.SeekFile) ([]byte, error) {
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	return io.ReadAll(file)
}

func (m *memStore) StoreFile(repo, arch string, file stream.SeekFile) error {
	b, err := readAllSeek(file)
	if err != nil {
		return err
	}
	m.mu.Lock()
	m.put(repo, arch, path.Base(file.FileName()), b)
	m.mu.Unlock()
	return nil
}

// FetchFileWithETag returns the file and its version, firing the one-shot onFetch
// hook (after capturing the version) to model a concurrent writer.
func (m *memStore) FetchFileWithETag(repo, arch, name string) (stream.File, string, error) {
	m.mu.Lock()
	k := m.keyOf(repo, arch, name)
	b, ok := m.files[k]
	etag := m.versions[k]
	hook := m.onFetch
	m.onFetch = nil
	ferr := m.fetchErr
	m.mu.Unlock()
	if hook != nil {
		hook(name)
	}
	if ferr != nil {
		return nil, "", ferr
	}
	if !ok {
		return nil, "", blob.ErrNotFound
	}
	return stream.NewFileStream(name, "application/octet-stream", nopSeekCloser{bytes.NewReader(b)}), etag, nil
}

// StoreFileIfMatch is the memStore compare-and-swap: it stores only when the live
// version equals etag (or, for etag=="", when the key is absent).
func (m *memStore) StoreFileIfMatch(repo, arch string, file stream.SeekFile, etag string) error {
	b, err := readAllSeek(file)
	if err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	k := m.keyOf(repo, arch, path.Base(file.FileName()))
	cur, exists := m.versions[k]
	if etag == "" {
		if exists {
			return blob.ErrPreconditionFailed
		}
	} else if cur != etag {
		return blob.ErrPreconditionFailed
	}
	m.put(repo, arch, path.Base(file.FileName()), b)
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
		return nil, blob.ErrNotFound
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

// TestRepoDBArtifactSet locks the stored artifact set: InitArch and RepoAdd store
// the .tar.gz archives through the blob.Store but NOT the bare .db/.files copies,
// which are served as aliases of the archives. It runs the orchestration against
// BOTH backends — the default Go-native writer (no repo-add needed) and the
// repo-add CLI (skipped when absent) — proving the fetch -> tool -> store cycle
// is backend-agnostic.
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

			want := []string{"r.db.tar.gz", "r.files.tar.gz"}
			mem := newMemStore()
			r := &binaryRepository{Store: mem, tool: be.tool}

			if err := r.InitArch("r", "x86_64", false, nil); err != nil {
				t.Fatalf("InitArch: %v", err)
			}
			assertSuperset(t, mem.names("r", "x86_64"), want, "InitArch")
			assertAliases(t, r, mem, "InitArch")

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
			assertAliases(t, r, mem, "RepoAdd")
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
			assertAliases(t, r, mem, "RepoRemove")
		})
	}
}

// assertAliases checks that the bare <repo>.db / <repo>.files are NOT stored, and
// that fetching them returns the byte-identical content of their .tar.gz archive.
func assertAliases(t *testing.T, r *binaryRepository, mem *memStore, ctx string) {
	t.Helper()
	stored := mem.names("r", "x86_64")
	for _, bare := range []string{"r.db", "r.files"} {
		if contains(stored, bare) {
			t.Errorf("%s: %q was stored; it must be served as an alias, not a copy", ctx, bare)
		}
	}
	aliases := map[string]string{"r.db": "r.db.tar.gz", "r.files": "r.files.tar.gz"}
	for bare, archive := range aliases {
		af, err := r.FetchFile("r", "x86_64", bare)
		if err != nil {
			t.Fatalf("%s: FetchFile(%q): %v", ctx, bare, err)
		}
		got, err := io.ReadAll(af)
		af.Close()
		if err != nil {
			t.Fatalf("%s: read alias %q: %v", ctx, bare, err)
		}
		want, err := mem.FetchFile("r", "x86_64", archive)
		if err != nil {
			t.Fatalf("%s: FetchFile(%q): %v", ctx, archive, err)
		}
		wantB, _ := io.ReadAll(want)
		want.Close()
		if !bytes.Equal(got, wantB) {
			t.Errorf("%s: alias %q does not match archive %q", ctx, bare, archive)
		}
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
func (fakeTool) RepoAddBatch(dbPath string, _ []string, _ bool, _ *string) error {
	return writeFakeQuartet(dbPath)
}
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
// binaryRepository orchestration stores the .tar.gz archives (serving .db/.files
// as aliases) and never stores the package file through the DB path. It needs no
// repo-add binary, so it runs everywhere.
func TestRepoDBToolPort(t *testing.T) {
	want := []string{"r.db.tar.gz", "r.files.tar.gz"}
	mem := newMemStore()
	r := &binaryRepository{Store: mem, tool: fakeTool{}}

	if err := r.InitArch("r", "x86_64", false, nil); err != nil {
		t.Fatalf("InitArch: %v", err)
	}
	assertSuperset(t, mem.names("r", "x86_64"), want, "InitArch")
	assertAliases(t, r, mem, "InitArch")

	pkg := stream.NewFileStream("foo-1.0-1-x86_64.pkg.tar.zst", "application/octet-stream",
		nopSeekCloser{bytes.NewReader([]byte("pkg"))})
	if err := r.RepoAdd("r", "x86_64", pkg, nil, false, nil); err != nil {
		t.Fatalf("RepoAdd: %v", err)
	}
	got := mem.names("r", "x86_64")
	assertSuperset(t, got, want, "RepoAdd")
	assertAliases(t, r, mem, "RepoAdd")
	if contains(got, "foo-1.0-1-x86_64.pkg.tar.zst") {
		t.Errorf("RepoAdd stored the package file through the DB path; the caller's StoreFile owns it")
	}

	if err := r.RepoRemove("r", "x86_64", "foo", false, nil); err != nil {
		t.Fatalf("RepoRemove: %v", err)
	}
	assertSuperset(t, mem.names("r", "x86_64"), want, "RepoRemove")
	assertAliases(t, r, mem, "RepoRemove")
}

// TestRepoDBCASNoLostUpdate proves the compare-and-swap retry preserves both
// concurrent additions: while one writer is mid-RepoAdd, another instance commits
// a different package; the first writer observes the conflict, re-reads, and
// re-applies, so the final database holds every package (no lost update).
func TestRepoDBCASNoLostUpdate(t *testing.T) {
	mem := newMemStore()
	// Two repositories share the store but hold independent dbMu locks, modelling
	// two server instances on a shared backend.
	r1 := &binaryRepository{Store: mem}
	r2 := &binaryRepository{Store: mem}

	if err := r1.InitArch("r", "x86_64", false, nil); err != nil {
		t.Fatalf("InitArch: %v", err)
	}
	dir := t.TempDir()
	addPkg := func(r *binaryRepository, name string) {
		t.Helper()
		p := makePkg(t, dir, name, "1.0-1", "x86_64")
		if err := r.RepoAdd("r", "x86_64", openSeek(t, p), nil, false, nil); err != nil {
			t.Fatalf("RepoAdd %s: %v", name, err)
		}
	}
	addPkg(r1, "alpha")

	// When r1 next reads the db, r2 sneaks "bravo" in first, so r1's CAS conflicts.
	mem.onFetch = func(name string) {
		if name == "r.db.tar.gz" {
			addPkg(r2, "bravo")
		}
	}
	addPkg(r1, "charlie")

	rr, err := r1.RemoteRepo("r", "x86_64")
	if err != nil {
		t.Fatalf("RemoteRepo: %v", err)
	}
	got := map[string]bool{}
	for _, p := range rr.Pkgs {
		got[p.Name()] = true
	}
	for _, name := range []string{"alpha", "bravo", "charlie"} {
		if !got[name] {
			t.Errorf("package %q lost to a concurrent write; db = %v", name, sortedKeys(got))
		}
	}
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// TestInitArchPreservesPopulatedDB proves InitArch never wipes a populated index.
// InitAll re-inits every (repo, arch) on every boot, so a re-init must be a no-op
// on an existing database, not an empty overwrite.
func TestInitArchPreservesPopulatedDB(t *testing.T) {
	mem := newMemStore()
	r := &binaryRepository{Store: mem}
	if err := r.InitArch("r", "x86_64", false, nil); err != nil {
		t.Fatalf("InitArch (create): %v", err)
	}
	dir := t.TempDir()
	if err := r.RepoAdd("r", "x86_64", openSeek(t, makePkg(t, dir, "foo", "1.0-1", "x86_64")), nil, false, nil); err != nil {
		t.Fatalf("RepoAdd: %v", err)
	}
	// Re-init, as InitAll does on the next boot.
	if err := r.InitArch("r", "x86_64", false, nil); err != nil {
		t.Fatalf("InitArch (re-init): %v", err)
	}
	rr, err := r.RemoteRepo("r", "x86_64")
	if err != nil {
		t.Fatalf("RemoteRepo: %v", err)
	}
	if len(rr.Pkgs) != 1 || rr.Pkgs[0].Name() != "foo" {
		t.Fatalf("re-init wiped the populated db: got %v, want [foo]", rr.Pkgs)
	}
}

// TestRepoRemoveIdempotent proves removing an already-absent package is a no-op
// success, so a retried remove after a partial failure does not error.
func TestRepoRemoveIdempotent(t *testing.T) {
	mem := newMemStore()
	r := &binaryRepository{Store: mem}
	if err := r.InitArch("r", "x86_64", false, nil); err != nil {
		t.Fatalf("InitArch: %v", err)
	}
	dir := t.TempDir()
	if err := r.RepoAdd("r", "x86_64", openSeek(t, makePkg(t, dir, "foo", "1.0-1", "x86_64")), nil, false, nil); err != nil {
		t.Fatalf("RepoAdd: %v", err)
	}
	if err := r.RepoRemove("r", "x86_64", "foo", false, nil); err != nil {
		t.Fatalf("RepoRemove: %v", err)
	}
	// A second remove of the same (now absent) package, and one that never existed,
	// must both succeed.
	if err := r.RepoRemove("r", "x86_64", "foo", false, nil); err != nil {
		t.Fatalf("idempotent re-remove: %v", err)
	}
	if err := r.RepoRemove("r", "x86_64", "ghost", false, nil); err != nil {
		t.Fatalf("remove of never-present package: %v", err)
	}
}

// TestRepoAddSurfacesTransientFetchError proves a transient backend error on the
// DB fetch is surfaced, not mistaken for "absent" — which would seed an empty base
// and overwrite the live db with a truncated rebuild.
func TestRepoAddSurfacesTransientFetchError(t *testing.T) {
	mem := newMemStore()
	r := &binaryRepository{Store: mem}
	if err := r.InitArch("r", "x86_64", false, nil); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := r.RepoAdd("r", "x86_64", openSeek(t, makePkg(t, dir, "foo", "1.0-1", "x86_64")), nil, false, nil); err != nil {
		t.Fatal(err)
	}

	boom := errors.New("transient backend error")
	mem.fetchErr = boom
	err := r.RepoAdd("r", "x86_64", openSeek(t, makePkg(t, dir, "bar", "1.0-1", "x86_64")), nil, false, nil)
	mem.fetchErr = nil
	if !errors.Is(err, boom) {
		t.Fatalf("transient fetch error should surface, got %v", err)
	}

	// The live db must be untouched: foo still present, bar never added.
	rr, _ := r.RemoteRepo("r", "x86_64")
	names := map[string]bool{}
	for _, p := range rr.Pkgs {
		names[p.Name()] = true
	}
	if !names["foo"] || names["bar"] {
		t.Fatalf("transient fetch error corrupted the db: %v (want only foo)", sortedKeys(names))
	}
}
