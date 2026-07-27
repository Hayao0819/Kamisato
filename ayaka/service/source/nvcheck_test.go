package source

import (
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
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

// makeAURCheckout turns the fixture dir into a git checkout whose origin points
// at aur.archlinux.org, which is what marks a package as an AUR mirror.
func makeAURCheckout(t *testing.T, p *pkg.SourcePackage) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	for _, args := range [][]string{
		{"init", "-q"},
		{"remote", "add", "origin", "https://aur.archlinux.org/" + p.Base() + ".git"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = p.Dir()
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
}

const fooSrcinfo = "pkgbase = foo\n\tpkgver = 1.0\n\tpkgrel = 2\n\tepoch = 1\n\tarch = any\n\npkgname = foo\n"

func TestRunNvCheck(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/pypi/foo/json":
			_, _ = w.Write([]byte(`{"info":{"version":"2.0"}}`))
		case "/rpc/v5/info":
			_, _ = w.Write([]byte(`{"results":[{"Version":"1.249-1"}]}`))
		default:
			http.NotFound(w, r)
		}
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
	// The bundled .nvchecker.toml is the AUR maintainer's upstream watch and
	// must lose to the mirror's own AUR RPC comparison.
	mirror := writeNvCheckFixture(t,
		"pkgbase = ckbcomp\n\tpkgver = 1.248\n\tpkgrel = 1\n\tarch = any\n\npkgname = ckbcomp\n",
		"[ckbcomp]\nsource = \"git\"\ngit = \"https://salsa.debian.org/x.git\"\n")
	makeAURCheckout(t, mirror)

	src := &repo.SourceRepo{
		Config: &repo.SrcConfig{Name: "test"},
		Pkgs:   []*pkg.SourcePackage{monitored, unmonitored, unsupported, broken, mirror},
	}

	client := &http.Client{Transport: rewriteHost(srv)}
	results := RunNvCheck(t.Context(), src, client)

	if len(results) != 4 {
		t.Fatalf("results = %d, want 4 (monitored, unsupported, broken, mirror)", len(results))
	}
	byPkg := map[string]int{}
	for i, r := range results {
		byPkg[r.Pkgbase] = i
	}

	foo := results[byPkg["foo"]]
	if foo.Err != nil || !foo.Outdated || foo.Latest != "2.0" || foo.Current != "1.0" || foo.Method != MethodNvBump {
		t.Errorf("foo = %+v, want nvbump-outdated 1.0 -> 2.0 (epoch/pkgrel excluded)", foo)
	}
	ckb := results[byPkg["ckbcomp"]]
	if ckb.Err != nil || !ckb.Outdated || ckb.Latest != "1.249-1" || ckb.Current != "1.248-1" || ckb.Method != MethodPull {
		t.Errorf("ckbcomp = %+v, want pull-outdated 1.248-1 -> 1.249-1 via AUR RPC", ckb)
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
