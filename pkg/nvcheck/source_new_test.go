package nvcheck

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"testing"
)

func TestArchpkgSource(t *testing.T) {
	body := `{"results":[
		{"repo":"core-testing","arch":"x86_64","pkgver":"9.9.9","pkgrel":"1","epoch":0},
		{"repo":"core","arch":"x86_64","pkgver":"7.1.5.arch1","pkgrel":"2","epoch":0},
		{"repo":"core","arch":"x86_64","pkgver":"7.1.4.arch1","pkgrel":"1","epoch":0}
	]}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/packages/search/json/" || r.URL.Query().Get("name") != "linux" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()

	for _, tt := range []struct {
		name  string
		strip bool
		want  string
	}{
		{"full version", false, "7.1.5.arch1-2"},
		{"strip_release", true, "7.1.5.arch1"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			src := &archpkgSource{pkg: "linux", stripRelease: tt.strip, base: srv.URL, client: srv.Client()}
			got, err := src.Latest(context.Background())
			if err != nil {
				t.Fatalf("Latest: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGitTagSource(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	dir := t.TempDir()
	for _, args := range [][]string{
		{"init", "-q"},
		{"-c", "user.name=t", "-c", "user.email=t@t", "commit", "-q", "--allow-empty", "-m", "init"},
		{"tag", "v1.2.0"},
		{"tag", "v1.10.0"},
		{"tag", "v1.9.0"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}

	src := &gitTagSource{url: dir, prefix: "v"}
	got, err := src.Latest(context.Background())
	if err != nil {
		t.Fatalf("Latest: %v", err)
	}
	if got != "1.10.0" {
		t.Errorf("got %q, want 1.10.0", got)
	}
}

func TestTransformSource(t *testing.T) {
	spec := Spec{
		Kind: "http", URL: "unused", Regex: "(unused)",
		FromPattern: `^(\d+\.\d+\.\d+)-(\d+)$`, ToPattern: `\1.\2`,
	}
	src, err := NewSource(spec, http.DefaultClient)
	if err != nil {
		t.Fatalf("NewSource: %v", err)
	}
	tr, ok := src.(*transformSource)
	if !ok {
		t.Fatalf("want transformSource, got %T", src)
	}
	tr.inner = staticSource("7.1.2-27")
	got, err := tr.Latest(context.Background())
	if err != nil {
		t.Fatalf("Latest: %v", err)
	}
	if got != "7.1.2.27" {
		t.Errorf("got %q, want 7.1.2.27", got)
	}
}

type staticSource string

func (s staticSource) Latest(context.Context) (string, error) { return string(s), nil }

func TestNewSourceGithubUseMaxTag(t *testing.T) {
	src, err := NewSource(Spec{Kind: "github", Repo: "o/r", UseMaxTag: true}, http.DefaultClient)
	if err != nil {
		t.Fatalf("NewSource: %v", err)
	}
	if _, ok := src.(*githubTagSource); !ok {
		t.Errorf("use_max_tag should select githubTagSource, got %T", src)
	}
}

func TestNewSourceFromPatternNeedsToPattern(t *testing.T) {
	if _, err := NewSource(Spec{Kind: "pypi", Package: "p", FromPattern: "x"}, http.DefaultClient); err == nil {
		t.Error("from_pattern without to_pattern should error")
	}
}

func TestWithGitHubToken(t *testing.T) {
	var got string
	base := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		got = req.Header.Get("Authorization")
		return &http.Response{StatusCode: http.StatusOK, Body: http.NoBody}, nil
	})}

	client := WithGitHubToken(base, "tok")
	req, _ := http.NewRequest(http.MethodGet, "https://api.github.com/repos/o/r/tags", nil)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if got != "Bearer tok" {
		t.Errorf("api.github.com auth = %q, want Bearer tok", got)
	}

	req, _ = http.NewRequest(http.MethodGet, "https://pypi.org/pypi/p/json", nil)
	resp, err = client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if got != "" {
		t.Errorf("non-github auth = %q, want empty", got)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }
