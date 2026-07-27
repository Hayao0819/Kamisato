package source

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	pkg "github.com/Hayao0819/Kamisato/pkg/pacman/pkg"
	"github.com/Hayao0819/Kamisato/pkg/pacman/repo"
)

func writeNvCheckFixture(t *testing.T, srcinfo, nvchecker string) *pkg.SourcePackage {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".SRCINFO"), []byte(srcinfo), 0o644); err != nil {
		t.Fatal(err)
	}
	if nvchecker != "" {
		if err := os.WriteFile(filepath.Join(dir, ".nvchecker.toml"), []byte(nvchecker), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	p, err := pkg.OpenSourcePackage(dir)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

const fooSrcinfo = "pkgbase = foo\n\tpkgver = 1.0\n\tpkgrel = 2\n\tepoch = 1\n\tarch = any\n\npkgname = foo\n"

func TestRunNvCheck(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/pypi/foo/json" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(`{"info":{"version":"2.0"}}`))
	}))
	defer srv.Close()

	monitored := writeNvCheckFixture(t, fooSrcinfo, "[foo]\nsource = \"pypi\"\n")
	unmonitored := writeNvCheckFixture(t,
		"pkgbase = bar\n\tpkgver = 1.0\n\tpkgrel = 1\n\tarch = any\n\npkgname = bar\n", "")
	unsupported := writeNvCheckFixture(t,
		"pkgbase = chrome\n\tpkgver = 1.0\n\tpkgrel = 1\n\tarch = any\n\npkgname = chrome\n",
		"[chrome]\nsource = \"apt\"\n")
	broken := writeNvCheckFixture(t,
		"pkgbase = baz\n\tpkgver = 1.0\n\tpkgrel = 1\n\tarch = any\n\npkgname = baz\n",
		"[unclosed\n")

	// The pypi source hits the real host unless redirected; point the fixture's
	// spec at the test server by rewriting the entry post-collection is not
	// possible, so use the test server via the http transport override below.
	src := &repo.SourceRepo{
		Config: &repo.SrcConfig{Name: "test"},
		Pkgs:   []*pkg.SourcePackage{monitored, unmonitored, unsupported, broken},
	}

	client := &http.Client{Transport: rewriteHost(srv)}
	results := RunNvCheck(t.Context(), src, client)

	if len(results) != 3 {
		t.Fatalf("results = %d, want 3 (monitored, unsupported, broken)", len(results))
	}
	byPkg := map[string]int{}
	for i, r := range results {
		byPkg[r.Pkgbase] = i
	}

	foo := results[byPkg["foo"]]
	if foo.Err != nil || !foo.Outdated || foo.Latest != "2.0" || foo.Current != "1.0" {
		t.Errorf("foo = %+v, want outdated 1.0 -> 2.0 (epoch/pkgrel excluded)", foo)
	}
	if chrome := results[byPkg["chrome"]]; chrome.Err == nil {
		t.Error("unsupported source should surface as an error result")
	}
	if baz := results[byPkg["baz"]]; baz.Err == nil {
		t.Error("broken .nvchecker.toml should surface as an error result")
	}
	if _, ok := byPkg["bar"]; ok {
		t.Error("package without .nvchecker.toml should be skipped")
	}
}

// rewriteHost sends every request to the test server regardless of host, so
// sources built with default upstream bases stay off the network.
func rewriteHost(srv *httptest.Server) http.RoundTripper {
	return roundTripFunc(func(req *http.Request) (*http.Response, error) {
		r := req.Clone(req.Context())
		r.URL.Scheme = "http"
		r.URL.Host = srv.Listener.Addr().String()
		return http.DefaultTransport.RoundTrip(r)
	})
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func TestCollectNvCheckEntriesSkipsUnmonitored(t *testing.T) {
	p := writeNvCheckFixture(t, fooSrcinfo, "")
	src := &repo.SourceRepo{Config: &repo.SrcConfig{Name: "test"}, Pkgs: []*pkg.SourcePackage{p}}
	if entries := CollectNvCheckEntries(src); len(entries) != 0 {
		t.Errorf("entries = %v, want none", entries)
	}
}
