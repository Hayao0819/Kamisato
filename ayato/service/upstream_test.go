package service_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/Hayao0819/Kamisato/ayato/domain"
	"github.com/Hayao0819/Kamisato/ayato/service"
	"github.com/Hayao0819/Kamisato/internal/conf"
	"github.com/Hayao0819/Kamisato/pkg/pacman/repo"
)

type pkgSpec struct{ name, ver string }

// buildRepoDB assembles an upstream .db / .files archive holding the given
// packages, using the same native writer ayato serves with.
func buildRepoDB(t *testing.T, repoName string, pkgs []pkgSpec) (dbGz, filesGz []byte) {
	t.Helper()
	dir := t.TempDir()
	var paths []string
	for _, p := range pkgs {
		fname := p.name + "-" + p.ver + "-x86_64.pkg.tar.zst"
		fp := filepath.Join(dir, fname)
		if err := os.WriteFile(fp, buildPackage(t, p.name, p.ver, "x86_64"), 0o644); err != nil {
			t.Fatalf("write pkg: %v", err)
		}
		paths = append(paths, fp)
	}
	dbPath := filepath.Join(dir, repoName+".db.tar.gz")
	if err := (repo.NativeTool{}).RepoAddBatch(dbPath, paths, false, nil); err != nil {
		t.Fatalf("build upstream db: %v", err)
	}
	dbGz, err := os.ReadFile(dbPath)
	if err != nil {
		t.Fatalf("read upstream db: %v", err)
	}
	filesGz, err = os.ReadFile(filepath.Join(dir, repoName+".files.tar.gz"))
	if err != nil {
		t.Fatalf("read upstream files: %v", err)
	}
	return dbGz, filesGz
}

// fakeUpstream is a mutable pacman mirror: it serves a repo database with an ETag
// and answers a matching If-None-Match with 304, so a test can exercise the
// conditional-GET no-op and an upstream change.
type fakeUpstream struct {
	mu      sync.Mutex
	etag    string
	dbGz    []byte
	filesGz []byte
	db200   int // number of full (200) db responses served
}

func (u *fakeUpstream) set(dbGz, filesGz []byte, etag string) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.dbGz, u.filesGz, u.etag = dbGz, filesGz, etag
}

func (u *fakeUpstream) handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u.mu.Lock()
		defer u.mu.Unlock()
		switch {
		case strings.HasSuffix(r.URL.Path, ".db"):
			if r.Header.Get("If-None-Match") == u.etag {
				w.WriteHeader(http.StatusNotModified)
				return
			}
			w.Header().Set("ETag", u.etag)
			u.db200++
			_, _ = w.Write(u.dbGz)
		case strings.HasSuffix(r.URL.Path, ".files"):
			w.Header().Set("ETag", u.etag+"-files")
			_, _ = w.Write(u.filesGz)
		default:
			http.NotFound(w, r)
		}
	}
}

func versionOf(t *testing.T, svc *service.Service, repo, arch, pkgname string) (string, bool) {
	t.Helper()
	pkgs, err := svc.Pkgs(repo, arch)
	if err != nil {
		t.Fatalf("Pkgs: %v", err)
	}
	for _, p := range pkgs.Packages {
		if p.PkgName == pkgname {
			return p.PkgVer, true
		}
	}
	return "", false
}

// TestUpstreamLayeringAndSync exercises the whole overlay model against a crafted
// upstream mirror: the merged view is upstream + local with local shadowing
// upstream on a name collision; an unchanged upstream is a conditional-GET no-op;
// an upstream change updates the merged view; and the local overlay survives a
// sync.
func TestUpstreamLayeringAndSync(t *testing.T) {
	up := &fakeUpstream{}
	db1, files1 := buildRepoDB(t, "extra", []pkgSpec{{"foo", "1.0-1"}, {"bar", "1.0-1"}})
	up.set(db1, files1, "v1")
	srv := httptest.NewServer(up.handler())
	defer srv.Close()

	svc, _, _ := newTieredService(t, []conf.BinRepoConfig{{
		Name:     "myrepo",
		Upstream: conf.UpstreamRepoConfig{DBURL: srv.URL + "/extra.db"},
	}})
	ctx := context.Background()

	// Publish a local overlay: bar shadows upstream at a higher version, baz is
	// local-only. (The version gate reads the local overlay, so shadowing upstream
	// is allowed regardless of the upstream version.)
	uploadVersioned(t, svc, "myrepo", "bar", "2.0-1")
	uploadVersioned(t, svc, "myrepo", "baz", "1.0-1")

	// First sync merges upstream in: the served view is upstream + local with local
	// winning on "bar".
	res, err := svc.SyncUpstream(ctx, "myrepo")
	if err != nil {
		t.Fatalf("SyncUpstream: %v", err)
	}
	if len(res.Arches) != 1 || !res.Arches[0].Changed {
		t.Fatalf("first sync = %+v, want one changed arch", res.Arches)
	}
	if v, ok := versionOf(t, svc, "myrepo", "x86_64", "foo"); !ok || v != "1.0-1" {
		t.Fatalf("merged foo = %q,%v, want 1.0-1 from upstream", v, ok)
	}
	if v, ok := versionOf(t, svc, "myrepo", "x86_64", "bar"); !ok || v != "2.0-1" {
		t.Fatalf("merged bar = %q,%v, want 2.0-1 (local shadows upstream)", v, ok)
	}
	if _, ok := versionOf(t, svc, "myrepo", "x86_64", "baz"); !ok {
		t.Fatal("merged view lost the local-only baz")
	}

	// A second sync with an unchanged upstream is a conditional-GET no-op (304):
	// no new full db body is served and the arch reports unchanged.
	before := up.db200
	res, err = svc.SyncUpstream(ctx, "myrepo")
	if err != nil {
		t.Fatalf("second SyncUpstream: %v", err)
	}
	if res.Arches[0].Changed {
		t.Error("unchanged upstream reported a change (conditional GET did not no-op)")
	}
	if up.db200 != before {
		t.Errorf("upstream served %d extra full db bodies on an unchanged sync, want 0", up.db200-before)
	}

	// The upstream changes (adds qux): the merged view gains qux while the local
	// overlay (bar 2.0-1 shadow, baz) survives.
	db2, files2 := buildRepoDB(t, "extra", []pkgSpec{{"foo", "1.0-1"}, {"bar", "1.0-1"}, {"qux", "1.0-1"}})
	up.set(db2, files2, "v2")
	res, err = svc.SyncUpstream(ctx, "myrepo")
	if err != nil {
		t.Fatalf("third SyncUpstream: %v", err)
	}
	if !res.Arches[0].Changed || res.Arches[0].Added != 1 {
		t.Fatalf("upstream-change sync = %+v, want changed with 1 added", res.Arches[0])
	}
	if _, ok := versionOf(t, svc, "myrepo", "x86_64", "qux"); !ok {
		t.Fatal("merged view did not pick up the new upstream qux")
	}
	if v, ok := versionOf(t, svc, "myrepo", "x86_64", "bar"); !ok || v != "2.0-1" {
		t.Fatalf("local shadow bar = %q,%v after upstream sync, want it to survive at 2.0-1", v, ok)
	}
	if _, ok := versionOf(t, svc, "myrepo", "x86_64", "baz"); !ok {
		t.Fatal("local-only baz did not survive the upstream sync")
	}
}

// TestSyncUpstreamRejectsNonUpstream proves sync is refused on a repo with no
// upstream configured.
func TestSyncUpstreamRejectsNonUpstream(t *testing.T) {
	svc, _, _ := newTieredService(t, []conf.BinRepoConfig{{Name: "plain"}})
	if _, err := svc.SyncUpstream(context.Background(), "plain"); err == nil {
		t.Fatal("SyncUpstream on a non-upstream repo = nil, want error")
	}
}

func TestSyncUpstreamKeepsAddressedPhysicalTier(t *testing.T) {
	up := &fakeUpstream{}
	db, files := buildRepoDB(t, "extra", []pkgSpec{{"foo", "1.0-1"}})
	up.set(db, files, "v1")
	srv := httptest.NewServer(up.handler())
	defer srv.Close()

	svc, _, _ := newTieredService(t, []conf.BinRepoConfig{{
		Name:     "myrepo",
		Tiered:   true,
		Arches:   []string{"x86_64"},
		Upstream: conf.UpstreamRepoConfig{DBURL: srv.URL + "/extra.db"},
	}})
	res, err := svc.SyncUpstream(context.Background(), "myrepo-testing")
	if err != nil {
		t.Fatalf("SyncUpstream(testing): %v", err)
	}
	if res.Repo != "myrepo-testing" {
		t.Fatalf("result repo = %q, want addressed physical tier", res.Repo)
	}
	if _, ok := versionOf(t, svc, "myrepo-testing", "x86_64", "foo"); !ok {
		t.Fatal("testing tier did not receive the upstream snapshot")
	}
	if _, ok := versionOf(t, svc, "myrepo", "x86_64", "foo"); ok {
		t.Fatal("syncing testing unexpectedly rewrote the stable tier")
	}
}

func TestSyncUpstreamRejectsOversizedResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Length", "536870913")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	svc, _, _ := newTieredService(t, []conf.BinRepoConfig{{
		Name: "myrepo", Upstream: conf.UpstreamRepoConfig{DBURL: srv.URL + "/extra.db"},
	}})
	uploadVersioned(t, svc, "myrepo", "local", "1.0-1")
	res, err := svc.SyncUpstream(context.Background(), "myrepo")
	if err != nil {
		t.Fatalf("SyncUpstream: %v", err)
	}
	if len(res.Arches) != 1 || !strings.Contains(res.Arches[0].Error, "exceeds") {
		t.Fatalf("result = %+v, want oversized-response error", res)
	}
}

func uploadVersioned(t *testing.T, svc *service.Service, repo, name, ver string) {
	t.Helper()
	fname := name + "-" + ver + "-x86_64.pkg.tar.zst"
	files := &domain.UploadFiles{PkgFile: pkgStream(fname, buildPackage(t, name, ver, "x86_64"))}
	if err := svc.UploadFile(repo, files); err != nil {
		t.Fatalf("upload %s %s: %v", name, ver, err)
	}
}
