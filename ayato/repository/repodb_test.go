package repository

import (
	"archive/tar"
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Hayao0819/Kamisato/internal/errors"

	"github.com/klauspost/compress/zstd"

	"github.com/Hayao0819/Kamisato/ayato/repository/blob"
	"github.com/Hayao0819/Kamisato/ayato/stream"
	"github.com/Hayao0819/Kamisato/pkg/pacman/repo"
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
	fetchErr          error
	storeErrName      string
	storeErr          error
	storeAfterErrName string
	storeAfterErr     error
	afterStoreName    string
	afterStore        func(name string)
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
	name := path.Base(file.FileName())
	m.mu.Lock()
	if name == m.storeErrName && m.storeErr != nil {
		m.mu.Unlock()
		return m.storeErr
	}
	k := m.keyOf(repo, arch, name)
	cur, exists := m.versions[k]
	if etag == "" {
		if exists {
			m.mu.Unlock()
			return blob.ErrPreconditionFailed
		}
	} else if cur != etag {
		m.mu.Unlock()
		return blob.ErrPreconditionFailed
	}
	m.put(repo, arch, name, b)
	afterErr := error(nil)
	if name == m.storeAfterErrName {
		afterErr = m.storeAfterErr
	}
	hook := m.afterStore
	if name != m.afterStoreName {
		hook = nil
	} else {
		m.afterStore = nil
	}
	m.mu.Unlock()
	if hook != nil {
		hook(name)
	}
	if afterErr != nil {
		return afterErr
	}
	return nil
}

func TestStoreFileImmutableReusesOnlyByteIdenticalObject(t *testing.T) {
	mem := newMemStore()
	r := &binaryRepository{Store: mem}
	file := func(body string) stream.SeekFile {
		return stream.NewFileStream("foo-1.0-1-x86_64.pkg.tar.zst", "application/octet-stream", nopSeekCloser{bytes.NewReader([]byte(body))})
	}
	created, err := r.StoreFileImmutable("r", "x86_64", file("winner"))
	if err != nil || !created {
		t.Fatalf("first create = (%v, %v), want true, nil", created, err)
	}
	stored, beforeVersion, err := mem.FetchFileWithETag("r", "x86_64", "foo-1.0-1-x86_64.pkg.tar.zst")
	if err != nil {
		t.Fatal(err)
	}
	_ = stored.Close()
	created, err = r.StoreFileImmutable("r", "x86_64", file("winner"))
	if err != nil || created {
		t.Fatalf("identical reuse = (%v, %v), want false, nil", created, err)
	}
	stored, afterVersion, err := mem.FetchFileWithETag("r", "x86_64", "foo-1.0-1-x86_64.pkg.tar.zst")
	if err != nil {
		t.Fatal(err)
	}
	_ = stored.Close()
	if beforeVersion == afterVersion {
		t.Fatal("identical reuse did not renew the object's version/lease")
	}
	if _, err := r.StoreFileImmutable("r", "x86_64", file("attacker")); !errors.Is(err, ErrImmutableObjectConflict) {
		t.Fatalf("different-content reuse = %v, want ErrImmutableObjectConflict", err)
	}
	stored, err = mem.FetchFile("r", "x86_64", "foo-1.0-1-x86_64.pkg.tar.zst")
	if err != nil {
		t.Fatal(err)
	}
	got, err := io.ReadAll(stored)
	_ = stored.Close()
	if err != nil || string(got) != "winner" {
		t.Fatalf("immutable object = %q, %v; want winner", got, err)
	}
}

func TestRepoAddConditionalRejectsConcurrentDowngrade(t *testing.T) {
	mem := newMemStore()
	r := &binaryRepository{Store: mem}
	if err := r.InitArch("r", "x86_64", false, nil); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	v0Path := makePkg(t, dir, "foo", "0.9-1", "x86_64")
	v0 := openSeek(t, v0Path)
	if err := r.RepoAdd("r", "x86_64", v0, nil, false, nil); err != nil {
		t.Fatal(err)
	}
	oldFile := path.Base(v0Path)
	v2 := openSeek(t, makePkg(t, dir, "foo", "2.0-1", "x86_64"))
	conditional := func(pkg stream.SeekFile) error {
		return r.RepoAddBatch("r", "x86_64", []RepoAddItem{{
			Pkg:                    pkg,
			CheckCurrent:           true,
			ExpectedName:           "foo",
			ExpectedCurrentVersion: "0.9-1",
			ExpectedCurrentFile:    oldFile,
		}}, false, nil)
	}
	if err := conditional(v2); err != nil {
		t.Fatal(err)
	}
	v1 := openSeek(t, makePkg(t, dir, "foo", "1.0-1", "x86_64"))
	if err := conditional(v1); !errors.Is(err, ErrPackageChanged) {
		t.Fatalf("stale v1 publish error = %v, want ErrPackageChanged", err)
	}
	rr, err := r.RemoteRepo("r", "x86_64")
	if err != nil {
		t.Fatal(err)
	}
	if got := rr.PkgByPkgName("foo").Version(); got != "2.0-1" {
		t.Fatalf("stale writer downgraded package to %s", got)
	}
}

func TestRepoAddReportsCanonicalCommitBeforeDerivedFailure(t *testing.T) {
	mem := newMemStore()
	r := &binaryRepository{Store: mem}
	if err := r.InitArch("r", "x86_64", false, nil); err != nil {
		t.Fatal(err)
	}
	boom := errors.New("files write failed")
	mem.storeErrName = "r.files.tar.gz"
	mem.storeErr = boom
	pkgPath := makePkg(t, t.TempDir(), "foo", "1.0-1", "x86_64")
	if err := mem.StoreFile("r", "x86_64", openSeek(t, pkgPath)); err != nil {
		t.Fatal(err)
	}
	err := r.RepoAdd("r", "x86_64", openSeek(t, pkgPath), nil, false, nil)
	if !CanonicalCommitted(err) || !errors.Is(err, boom) {
		t.Fatalf("RepoAdd error = %v, want canonical-committed wrapper around boom", err)
	}
	mem.storeErr = nil
	rr, fetchErr := r.RemoteRepo("r", "x86_64")
	if fetchErr != nil {
		t.Fatal(fetchErr)
	}
	if rr.PkgByPkgName("foo") == nil {
		t.Fatal("canonical DB did not expose the committed package")
	}
	if err := r.InitArch("r", "x86_64", false, nil); err != nil {
		t.Fatalf("startup reconciliation: %v", err)
	}
	files, err := mem.FetchFile("r", "x86_64", "r.files.tar.gz")
	if err != nil {
		t.Fatal(err)
	}
	filesRepo, err := repo.RemoteRepoFromDB("r", files)
	_ = files.Close()
	if err != nil || filesRepo.PkgByPkgName("foo") == nil {
		t.Fatalf("reconciled files DB = %v, %v; want foo", filesRepo, err)
	}
}

func TestRepoAddTreatsCanonicalWriteResponseFailureAsCommitted(t *testing.T) {
	mem := newMemStore()
	r := &binaryRepository{Store: mem}
	if err := r.InitArch("r", "x86_64", false, nil); err != nil {
		t.Fatal(err)
	}
	boom := errors.New("response lost after canonical write")
	mem.storeAfterErrName = "r.db.tar.gz"
	mem.storeAfterErr = boom
	pkgPath := makePkg(t, t.TempDir(), "foo", "1.0-1", "x86_64")
	err := r.RepoAdd("r", "x86_64", openSeek(t, pkgPath), nil, false, nil)
	if !CanonicalCommitted(err) || !errors.Is(err, boom) {
		t.Fatalf("RepoAdd error = %v, want canonical-committed ambiguous outcome", err)
	}
	rr, fetchErr := r.RemoteRepo("r", "x86_64")
	if fetchErr != nil || rr.PkgByPkgName("foo") == nil {
		t.Fatalf("ambiguous write did not leave visible canonical foo: %v, %v", rr, fetchErr)
	}
}

func TestReconcileDBNeverImportsStaleFilesIntoCanonical(t *testing.T) {
	mem := newMemStore()
	r := &binaryRepository{Store: mem}
	if err := r.InitArch("r", "x86_64", false, nil); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	v1 := makePkg(t, dir, "foo", "1.0-1", "x86_64")
	if err := mem.StoreFile("r", "x86_64", openSeek(t, v1)); err != nil {
		t.Fatal(err)
	}
	if err := r.RepoAdd("r", "x86_64", openSeek(t, v1), nil, false, nil); err != nil {
		t.Fatal(err)
	}
	staleStream, err := mem.FetchFile("r", "x86_64", "r.files.tar.gz")
	if err != nil {
		t.Fatal(err)
	}
	staleFiles, err := io.ReadAll(staleStream)
	_ = staleStream.Close()
	if err != nil {
		t.Fatal(err)
	}

	v2 := makePkg(t, dir, "foo", "2.0-1", "x86_64")
	if err := mem.StoreFile("r", "x86_64", openSeek(t, v2)); err != nil {
		t.Fatal(err)
	}
	if err := r.RepoAdd("r", "x86_64", openSeek(t, v2), nil, false, nil); err != nil {
		t.Fatal(err)
	}
	canonical, beforeVersion, err := mem.FetchFileWithETag("r", "x86_64", "r.db.tar.gz")
	if err != nil {
		t.Fatal(err)
	}
	_ = canonical.Close()
	if err := mem.StoreFile("r", "x86_64", stream.NewFileStream("r.files.tar.gz", "application/octet-stream", nopSeekCloser{bytes.NewReader(staleFiles)})); err != nil {
		t.Fatal(err)
	}

	if err := r.ReconcileDB("r", "x86_64", false, nil); err != nil {
		t.Fatal(err)
	}
	canonical, afterVersion, err := mem.FetchFileWithETag("r", "x86_64", "r.db.tar.gz")
	if err != nil {
		t.Fatal(err)
	}
	canonicalRepo, err := repo.RemoteRepoFromDB("r", canonical)
	_ = canonical.Close()
	if err != nil {
		t.Fatal(err)
	}
	if beforeVersion != afterVersion {
		t.Fatalf("reconcile rewrote canonical version: before=%s after=%s", beforeVersion, afterVersion)
	}
	if len(canonicalRepo.Pkgs) != 1 || canonicalRepo.Pkgs[0].Version() != "2.0-1" {
		t.Fatalf("canonical after reconcile = %v, want foo v2 only", canonicalRepo.Pkgs)
	}
	files, err := mem.FetchFile("r", "x86_64", "r.files.tar.gz")
	if err != nil {
		t.Fatal(err)
	}
	filesRepo, err := repo.RemoteRepoFromDB("r", files)
	_ = files.Close()
	if err != nil {
		t.Fatal(err)
	}
	if len(filesRepo.Pkgs) != 1 || filesRepo.Pkgs[0].Version() != "2.0-1" {
		t.Fatalf("derived files after reconcile = %v, want foo v2 only", filesRepo.Pkgs)
	}

	if err := r.RepoRemove("r", "x86_64", "foo", false, nil); err != nil {
		t.Fatal(err)
	}
	canonical, beforeVersion, err = mem.FetchFileWithETag("r", "x86_64", "r.db.tar.gz")
	if err != nil {
		t.Fatal(err)
	}
	_ = canonical.Close()
	if err := mem.StoreFile("r", "x86_64", stream.NewFileStream("r.files.tar.gz", "application/octet-stream", nopSeekCloser{bytes.NewReader(staleFiles)})); err != nil {
		t.Fatal(err)
	}
	if err := r.ReconcileDB("r", "x86_64", false, nil); err != nil {
		t.Fatal(err)
	}
	canonical, afterVersion, err = mem.FetchFileWithETag("r", "x86_64", "r.db.tar.gz")
	if err != nil {
		t.Fatal(err)
	}
	emptyCanonical, err := repo.RemoteRepoFromDB("r", canonical)
	_ = canonical.Close()
	if err != nil {
		t.Fatal(err)
	}
	if beforeVersion != afterVersion || len(emptyCanonical.Pkgs) != 0 {
		t.Fatalf("stale files resurrected deleted package or rewrote canonical: version %s -> %s, pkgs=%v", beforeVersion, afterVersion, emptyCanonical.Pkgs)
	}
	files, err = mem.FetchFile("r", "x86_64", "r.files.tar.gz")
	if err != nil {
		t.Fatal(err)
	}
	emptyFiles, err := repo.RemoteRepoFromDB("r", files)
	_ = files.Close()
	if err != nil || len(emptyFiles.Pkgs) != 0 {
		t.Fatalf("derived files retained deleted package: %v, %v", emptyFiles, err)
	}
}

func TestReconcileDBDiscardsCorruptDerivedFiles(t *testing.T) {
	mem := newMemStore()
	r := &binaryRepository{Store: mem}
	if err := r.InitArch("r", "x86_64", false, nil); err != nil {
		t.Fatal(err)
	}
	pkgPath := makePkg(t, t.TempDir(), "foo", "1.0-1", "x86_64")
	if err := mem.StoreFile("r", "x86_64", openSeek(t, pkgPath)); err != nil {
		t.Fatal(err)
	}
	if err := r.RepoAdd("r", "x86_64", openSeek(t, pkgPath), nil, false, nil); err != nil {
		t.Fatal(err)
	}

	canonical, beforeVersion, err := mem.FetchFileWithETag("r", "x86_64", "r.db.tar.gz")
	if err != nil {
		t.Fatal(err)
	}
	_ = canonical.Close()
	corrupt := stream.NewFileStream("r.files.tar.gz", "application/octet-stream", nopSeekCloser{bytes.NewReader([]byte{0x1f, 0x8b, 0x08, 0x00})})
	if err := mem.StoreFile("r", "x86_64", corrupt); err != nil {
		t.Fatal(err)
	}

	if err := r.ReconcileDB("r", "x86_64", false, nil); err != nil {
		t.Fatalf("ReconcileDB with corrupt .files: %v", err)
	}
	canonical, afterVersion, err := mem.FetchFileWithETag("r", "x86_64", "r.db.tar.gz")
	if err != nil {
		t.Fatal(err)
	}
	canonicalRepo, err := repo.RemoteRepoFromDB("r", canonical)
	_ = canonical.Close()
	if err != nil {
		t.Fatal(err)
	}
	if beforeVersion != afterVersion {
		t.Fatalf("derived repair rewrote canonical DB: %s -> %s", beforeVersion, afterVersion)
	}
	if canonicalRepo.PkgByPkgName("foo") == nil {
		t.Fatalf("canonical package disappeared during derived repair: %v", canonicalRepo.Pkgs)
	}

	files, err := mem.FetchFile("r", "x86_64", "r.files.tar.gz")
	if err != nil {
		t.Fatal(err)
	}
	filesRepo, err := repo.RemoteRepoFromDB("r", files)
	_ = files.Close()
	if err != nil || filesRepo.PkgByPkgName("foo") == nil {
		t.Fatalf("repaired .files = %v, %v; want foo", filesRepo, err)
	}
}

func TestConcurrentWritersReconcileDerivedArtifactsToCanonicalDB(t *testing.T) {
	mem := newMemStore()
	r1 := &binaryRepository{Store: mem}
	r2 := &binaryRepository{Store: mem}
	if err := r1.InitArch("r", "x86_64", false, nil); err != nil {
		t.Fatal(err)
	}

	aCanonical := make(chan struct{})
	releaseA := make(chan struct{})
	var releaseOnce sync.Once
	defer releaseOnce.Do(func() { close(releaseA) })
	mem.mu.Lock()
	mem.afterStoreName = "r.db.tar.gz"
	mem.afterStore = func(string) {
		close(aCanonical)
		<-releaseA
	}
	mem.mu.Unlock()

	dir := t.TempDir()
	aPkg := openSeek(t, makePkg(t, dir, "alpha", "1.0-1", "x86_64"))
	bPkg := openSeek(t, makePkg(t, dir, "bravo", "1.0-1", "x86_64"))
	aDone := make(chan error, 1)
	go func() {
		aDone <- r1.RepoAddBatch("r", "x86_64", []RepoAddItem{{
			Pkg:             aPkg,
			CheckCurrent:    true,
			ExpectedName:    "alpha",
			IntendedVersion: "1.0-1",
			IntendedFile:    "alpha-1.0-1-x86_64.pkg.tar.zst",
		}}, false, nil)
	}()

	select {
	case <-aCanonical:
	case <-time.After(5 * time.Second):
		t.Fatal("writer A did not reach its canonical commit")
	}

	bDone := make(chan error, 1)
	go func() { bDone <- r2.RepoAdd("r", "x86_64", bPkg, nil, false, nil) }()
	select {
	case err := <-bDone:
		if err != nil {
			releaseOnce.Do(func() { close(releaseA) })
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		releaseOnce.Do(func() { close(releaseA) })
		t.Fatal("writer B did not complete while writer A was paused")
	}
	releaseOnce.Do(func() { close(releaseA) })
	select {
	case err := <-aDone:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("writer A did not reconcile after the conflict")
	}

	for _, artifact := range []string{"r.db.tar.gz", "r.files.tar.gz"} {
		f, err := mem.FetchFile("r", "x86_64", artifact)
		if err != nil {
			t.Fatalf("fetch %s: %v", artifact, err)
		}
		rr, err := repo.RemoteRepoFromDB("r", f)
		_ = f.Close()
		if err != nil {
			t.Fatalf("parse %s: %v", artifact, err)
		}
		for _, name := range []string{"alpha", "bravo"} {
			if rr.PkgByPkgName(name) == nil {
				t.Fatalf("%s is stale: missing %s", artifact, name)
			}
		}
	}
}

func TestPostCanonicalSupersessionReturnsCommittedOutcome(t *testing.T) {
	mem := newMemStore()
	r1 := &binaryRepository{Store: mem}
	r2 := &binaryRepository{Store: mem}
	if err := r1.InitArch("r", "x86_64", false, nil); err != nil {
		t.Fatal(err)
	}

	aCanonical := make(chan struct{})
	releaseA := make(chan struct{})
	var releaseOnce sync.Once
	defer releaseOnce.Do(func() { close(releaseA) })
	mem.mu.Lock()
	mem.afterStoreName = "r.db.tar.gz"
	mem.afterStore = func(string) {
		close(aCanonical)
		<-releaseA
	}
	mem.mu.Unlock()

	dir := t.TempDir()
	alpha1 := openSeek(t, makePkg(t, dir, "alpha", "1.0-1", "x86_64"))
	charlie1 := openSeek(t, makePkg(t, dir, "charlie", "1.0-1", "x86_64"))
	aDone := make(chan error, 1)
	go func() {
		aDone <- r1.RepoAddBatch("r", "x86_64", []RepoAddItem{
			{Pkg: alpha1, CheckCurrent: true, ExpectedName: "alpha", IntendedVersion: "1.0-1", IntendedFile: "alpha-1.0-1-x86_64.pkg.tar.zst"},
			{Pkg: charlie1, CheckCurrent: true, ExpectedName: "charlie", IntendedVersion: "1.0-1", IntendedFile: "charlie-1.0-1-x86_64.pkg.tar.zst"},
		}, false, nil)
	}()
	select {
	case <-aCanonical:
	case <-time.After(5 * time.Second):
		t.Fatal("writer A did not commit canonical DB")
	}

	alpha2 := openSeek(t, makePkg(t, dir, "alpha", "2.0-1", "x86_64"))
	if err := r2.RepoAddBatch("r", "x86_64", []RepoAddItem{{
		Pkg:                    alpha2,
		CheckCurrent:           true,
		ExpectedName:           "alpha",
		ExpectedCurrentVersion: "1.0-1",
		ExpectedCurrentFile:    "alpha-1.0-1-x86_64.pkg.tar.zst",
		IntendedVersion:        "2.0-1",
		IntendedFile:           "alpha-2.0-1-x86_64.pkg.tar.zst",
	}}, false, nil); err != nil {
		releaseOnce.Do(func() { close(releaseA) })
		t.Fatal(err)
	}
	releaseOnce.Do(func() { close(releaseA) })
	select {
	case err := <-aDone:
		if !CanonicalCommitted(err) || !errors.Is(err, ErrPackageChanged) {
			t.Fatalf("writer A error = %v, want canonical-committed ErrPackageChanged", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("writer A did not finish after supersession")
	}

	rr, err := r1.RemoteRepo("r", "x86_64")
	if err != nil {
		t.Fatal(err)
	}
	if alpha := rr.PkgByPkgName("alpha"); alpha == nil || alpha.Version() != "2.0-1" {
		t.Fatalf("newer alpha was not retained: %v", alpha)
	}
	if rr.PkgByPkgName("charlie") == nil {
		t.Fatal("writer B's canonical base should include writer A's other package")
	}
}

func (m *memStore) StoreFileWithSignedURL(string, string, string) (string, error)    { return "", nil }
func (m *memStore) StoreFileWithSignedPutURL(string, string, string) (string, error) { return "", nil }
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

func (m *memStore) RepoNames() ([]string, error)                          { return nil, nil }
func (m *memStore) Files(string, string) ([]string, error)                { return nil, nil }
func (m *memStore) FilesWithMeta(string, string) ([]blob.FileInfo, error) { return nil, nil }
func (m *memStore) Arches(string) ([]string, error)                       { return nil, nil }

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

// fakeTool stands in for the repo-add/repo-remove CLI: it writes the canonical DB
// quartet next to the db path, so the binaryRepository orchestration (fetch -> tool
// -> store) can be exercised without the repo-add binary.
type fakeTool struct{}

func (fakeTool) RepoAdd(dbPath, _ string, _ bool, _ *string) error { return writeFakeQuartet(dbPath) }
func (fakeTool) RepoAddBatch(dbPath string, _ []string, _ bool, _ *string) error {
	return writeFakeQuartet(dbPath)
}
func (fakeTool) RebuildDerived(dbPath string, _ []string, _ bool, _ *string) error {
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
