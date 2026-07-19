package service_test

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/Hayao0819/Kamisato/ayato/domain"
	"github.com/Hayao0819/Kamisato/ayato/service"
	"github.com/Hayao0819/Kamisato/pkg/pacman/repo"
)

type pkgSpec struct {
	name string
	ver  string
}

func buildRepoDB(t *testing.T, repoName string, packages []pkgSpec) (db, files []byte) {
	t.Helper()
	dir := t.TempDir()
	paths := make([]string, 0, len(packages))
	for _, pkg := range packages {
		filename := pkg.name + "-" + pkg.ver + "-x86_64.pkg.tar.zst"
		path := filepath.Join(dir, filename)
		if err := os.WriteFile(
			path,
			buildPackage(t, pkg.name, pkg.ver, "x86_64"),
			0o644,
		); err != nil {
			t.Fatalf("write package: %v", err)
		}
		paths = append(paths, path)
	}

	dbPath := filepath.Join(dir, repoName+".db.tar.gz")
	if err := (repo.NativeTool{}).RepoAddBatch(dbPath, paths, false, nil); err != nil {
		t.Fatalf("build upstream db: %v", err)
	}
	db, err := os.ReadFile(dbPath)
	if err != nil {
		t.Fatalf("read upstream db: %v", err)
	}
	files, err = os.ReadFile(filepath.Join(dir, repoName+".files.tar.gz"))
	if err != nil {
		t.Fatalf("read upstream files: %v", err)
	}
	return db, files
}

type fakeUpstream struct {
	mu      sync.Mutex
	etag    string
	db      []byte
	files   []byte
	dbReads int
}

func (upstream *fakeUpstream) set(db, files []byte, etag string) {
	upstream.mu.Lock()
	defer upstream.mu.Unlock()
	upstream.db = db
	upstream.files = files
	upstream.etag = etag
}

func (upstream *fakeUpstream) dbReadCount() int {
	upstream.mu.Lock()
	defer upstream.mu.Unlock()
	return upstream.dbReads
}

func (upstream *fakeUpstream) handler() http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		upstream.mu.Lock()
		defer upstream.mu.Unlock()
		switch {
		case strings.HasSuffix(request.URL.Path, ".db"):
			if request.Header.Get("If-None-Match") == upstream.etag {
				response.WriteHeader(http.StatusNotModified)
				return
			}
			response.Header().Set("ETag", upstream.etag)
			upstream.dbReads++
			_, _ = response.Write(upstream.db)
		case strings.HasSuffix(request.URL.Path, ".files"):
			response.Header().Set("ETag", upstream.etag+"-files")
			_, _ = response.Write(upstream.files)
		default:
			http.NotFound(response, request)
		}
	}
}

func versionOf(
	t *testing.T,
	svc *service.Service,
	repoName,
	arch,
	packageName string,
) (string, bool) {
	t.Helper()
	packages, err := svc.Pkgs(repoName, arch)
	if err != nil {
		t.Fatalf("Pkgs: %v", err)
	}
	for _, pkg := range packages.Packages {
		if pkg.PkgName == packageName {
			return pkg.PkgVer, true
		}
	}
	return "", false
}

func uploadVersioned(t *testing.T, svc *service.Service, repoName, name, version string) {
	t.Helper()
	filename := name + "-" + version + "-x86_64.pkg.tar.zst"
	files := &domain.UploadFiles{
		PkgFile: pkgStream(filename, buildPackage(t, name, version, "x86_64")),
	}
	if err := svc.UploadFile(repoName, files); err != nil {
		t.Fatalf("upload %s %s: %v", name, version, err)
	}
}
