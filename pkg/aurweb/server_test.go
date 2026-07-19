package aurweb

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// stubBackend is an in-memory Backend for tests.
type stubBackend struct {
	pkgs    map[string]Pkg // by name
	sources map[string]string
}

func (b *stubBackend) Info(_ context.Context, names []string) ([]Pkg, error) {
	var out []Pkg
	for _, n := range names {
		if p, ok := b.pkgs[n]; ok {
			out = append(out, p)
		}
	}
	return out, nil
}

func (b *stubBackend) Search(_ context.Context, by By, arg string) ([]Pkg, error) {
	var out []Pkg
	for _, p := range b.pkgs {
		if strings.Contains(p.Name, arg) {
			out = append(out, p)
		}
	}
	return out, nil
}

func (b *stubBackend) Suggest(_ context.Context, arg string, _ bool) ([]string, error) {
	var out []string
	for name := range b.pkgs {
		if strings.HasPrefix(name, arg) {
			out = append(out, name)
		}
	}
	return out, nil
}

func (b *stubBackend) SourceURL(_ context.Context, pkgbase string) (string, bool, error) {
	if u, ok := b.sources[pkgbase]; ok {
		return u, true, nil
	}
	return "", false, nil
}

func (b *stubBackend) All(_ context.Context) ([]Pkg, error) {
	out := make([]Pkg, 0, len(b.pkgs))
	for _, p := range b.pkgs {
		out = append(out, p)
	}
	return out, nil
}

func newTestServer() *Server {
	be := &stubBackend{
		pkgs: map[string]Pkg{
			"mytool": {
				ID: 1, Name: "mytool", PackageBase: "mytool", PackageBaseID: 1,
				Version: "1.0.0-1", Description: "a tool", Maintainer: "me",
				URLPath: "/cgit/aur.git/snapshot/mytool.tar.gz",
				Depends: []string{"glibc"}, License: []string{"MIT"},
			},
		},
		sources: map[string]string{"mytool": "https://git.example.com/mytool.git"},
	}
	return New(be)
}

func doRPC(t *testing.T, s *Server, target string) map[string]any {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, target, nil)
	rec := httptest.NewRecorder()
	s.RPC(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var got map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v (body=%s)", err, rec.Body.String())
	}
	return got
}

func TestInfoEnvelope(t *testing.T) {
	s := newTestServer()
	got := doRPC(t, s, "/rpc?v=5&type=info&arg[]=mytool")

	if got["type"] != "multiinfo" {
		t.Errorf("type = %v, want multiinfo", got["type"])
	}
	if got["version"].(float64) != 5 {
		t.Errorf("version = %v, want 5", got["version"])
	}
	if got["resultcount"].(float64) != 1 {
		t.Fatalf("resultcount = %v, want 1", got["resultcount"])
	}
	res := got["results"].([]any)[0].(map[string]any)
	if res["Name"] != "mytool" {
		t.Errorf("Name = %v", res["Name"])
	}
	// info carries dependency arrays and Submitter/License/Keywords.
	if _, ok := res["Depends"]; !ok {
		t.Errorf("info result missing Depends")
	}
	if _, ok := res["License"]; !ok {
		t.Errorf("info result missing License (must always be present)")
	}
	if _, ok := res["Submitter"]; !ok {
		t.Errorf("info result missing Submitter")
	}
	// OutOfDate not flagged -> null.
	if res["OutOfDate"] != nil {
		t.Errorf("OutOfDate = %v, want null", res["OutOfDate"])
	}
}

func TestSearchStripsInfoFields(t *testing.T) {
	s := newTestServer()
	got := doRPC(t, s, "/rpc?v=5&type=search&arg=mytool")

	if got["type"] != "search" {
		t.Errorf("type = %v, want search", got["type"])
	}
	res := got["results"].([]any)[0].(map[string]any)
	for _, banned := range []string{"Depends", "License", "Keywords", "Submitter"} {
		if _, ok := res[banned]; ok {
			t.Errorf("search result must not contain %q", banned)
		}
	}
	if res["Name"] != "mytool" {
		t.Errorf("Name = %v", res["Name"])
	}
}

func TestSearchMinLength(t *testing.T) {
	s := newTestServer()
	got := doRPC(t, s, "/rpc?v=5&type=search&arg=a")
	if got["type"] != "error" || got["error"] != "Query arg too small." {
		t.Errorf("got %v, want too-small error", got)
	}
}

func TestVersionValidation(t *testing.T) {
	s := newTestServer()
	if got := doRPC(t, s, "/rpc?v=4&type=search&arg=mytool"); got["error"] != "Invalid version specified." {
		t.Errorf("v=4 error = %v", got["error"])
	}
	if got := doRPC(t, s, "/rpc?type=search&arg=mytool"); got["error"] != "Please specify an API version." {
		t.Errorf("missing version error = %v", got["error"])
	}
}

func TestSuggestBareArray(t *testing.T) {
	s := newTestServer()
	req := httptest.NewRequest(http.MethodGet, "/rpc?v=5&type=suggest&arg=my", nil)
	rec := httptest.NewRecorder()
	s.RPC(rec, req)
	var names []string
	if err := json.Unmarshal(rec.Body.Bytes(), &names); err != nil {
		t.Fatalf("suggest must be a bare array: %v (body=%s)", err, rec.Body.String())
	}
	if len(names) != 1 || names[0] != "mytool" {
		t.Errorf("suggest = %v", names)
	}
}

func TestOpenAPIInfoPath(t *testing.T) {
	s := newTestServer()
	got := doRPC(t, s, "/rpc/v5/info?arg[]=mytool")
	if got["resultcount"].(float64) != 1 {
		t.Errorf("openapi info resultcount = %v", got["resultcount"])
	}
}

func TestOpenAPIQueryArgWinsOverPathArg(t *testing.T) {
	s := newTestServer()
	// Path arg is a fallback only; a query arg takes precedence (aurweb behavior).
	got := doRPC(t, s, "/rpc/v5/search/zzz?arg=mytool")
	if got["resultcount"].(float64) != 1 {
		t.Errorf("query arg should win: resultcount = %v", got["resultcount"])
	}
	got = doRPC(t, s, "/rpc/v5/search/mytool")
	if got["resultcount"].(float64) != 1 {
		t.Errorf("path arg fallback failed: resultcount = %v", got["resultcount"])
	}
}

func TestMetaDumpLocal(t *testing.T) {
	s := newTestServer()
	req := httptest.NewRequest(http.MethodGet, "/packages-meta-ext-v1.json.gz", nil)
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}

	gz, err := gzip.NewReader(rec.Body)
	if err != nil {
		t.Fatalf("response is not gzip: %v", err)
	}
	data, _ := io.ReadAll(gz)
	var arr []map[string]any
	if err := json.Unmarshal(data, &arr); err != nil {
		t.Fatalf("dump not valid json: %v (%s)", err, data)
	}
	if len(arr) != 1 || arr[0]["Name"] != "mytool" {
		t.Fatalf("dump = %v", arr)
	}
	if _, ok := arr[0]["Depends"]; !ok {
		t.Errorf("ext dump must carry Depends")
	}
}

func TestNamesDumpLocal(t *testing.T) {
	s := newTestServer()
	req := httptest.NewRequest(http.MethodGet, "/packages.gz", nil)
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)

	gz, err := gzip.NewReader(rec.Body)
	if err != nil {
		t.Fatalf("response is not gzip: %v", err)
	}
	data, _ := io.ReadAll(gz)
	if strings.TrimSpace(string(data)) != "mytool" {
		t.Errorf("names dump = %q", data)
	}
}

func TestNotModified(t *testing.T) {
	s := newTestServer()
	req := httptest.NewRequest(http.MethodGet, "/rpc?v=5&type=info&arg[]=mytool", nil)
	rec := httptest.NewRecorder()
	s.RPC(rec, req)
	etag := rec.Header().Get("ETag")
	if etag == "" {
		t.Fatal("no ETag")
	}

	req2 := httptest.NewRequest(http.MethodGet, "/rpc?v=5&type=info&arg[]=mytool", nil)
	req2.Header.Set("If-None-Match", etag)
	rec2 := httptest.NewRecorder()
	s.RPC(rec2, req2)
	if rec2.Code != http.StatusNotModified {
		t.Errorf("status = %d, want 304", rec2.Code)
	}
}

func TestGitCloneRedirect(t *testing.T) {
	s := newTestServer()
	req := httptest.NewRequest(http.MethodGet, "/mytool.git/info/refs?service=git-upload-pack", nil)
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)
	if rec.Code != http.StatusFound {
		t.Fatalf("status = %d, want 302", rec.Code)
	}
	want := "https://git.example.com/mytool.git/info/refs?service=git-upload-pack"
	if loc := rec.Header().Get("Location"); loc != want {
		t.Errorf("Location = %q, want %q", loc, want)
	}
}

// fakeUpstream records calls and returns canned data.
type fakeUpstream struct {
	info   []Pkg
	search []Pkg
}

func (f *fakeUpstream) Info(_ context.Context, _ []string) ([]Pkg, error) { return f.info, nil }
func (f *fakeUpstream) Search(_ context.Context, _ By, _ string) ([]Pkg, error) {
	return f.search, nil
}

func (f *fakeUpstream) Suggest(_ context.Context, _ string, _ bool) ([]string, error) {
	return []string{"upstreampkg"}, nil
}
func (f *fakeUpstream) GitBase() string             { return "https://aur.archlinux.org" }
func (f *fakeUpstream) SnapshotURL(b string) string { return "https://aur/snap/" + b }
func (f *fakeUpstream) PlainURL(n string) string    { return "https://aur/plain/" + n }

func TestUpstreamFallbackInfo(t *testing.T) {
	be := &stubBackend{pkgs: map[string]Pkg{"local": {Name: "local", PackageBase: "local"}}}
	up := &fakeUpstream{info: []Pkg{{Name: "remote", PackageBase: "remote"}}}
	s := New(be, WithUpstream(up))

	got := doRPC(t, s, "/rpc?v=5&type=info&arg[]=local&arg[]=remote")
	if got["resultcount"].(float64) != 2 {
		t.Fatalf("resultcount = %v, want 2 (local + upstream)", got["resultcount"])
	}
}

func TestUpstreamGitFallbackRedirect(t *testing.T) {
	be := &stubBackend{pkgs: map[string]Pkg{}}
	up := &fakeUpstream{}
	s := New(be, WithUpstream(up))

	req := httptest.NewRequest(http.MethodGet, "/unknown.git/info/refs?service=git-upload-pack", nil)
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)
	want := "https://aur.archlinux.org/unknown.git/info/refs?service=git-upload-pack"
	if loc := rec.Header().Get("Location"); loc != want {
		t.Errorf("Location = %q, want %q", loc, want)
	}
}
