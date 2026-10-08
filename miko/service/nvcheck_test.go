package service

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Hayao0819/Kamisato/internal/pacman/nvcheck"
	ppkg "github.com/Hayao0819/Kamisato/internal/pacman/pkg"
	"github.com/Hayao0819/Kamisato/internal/pacman/repo"
	"github.com/Hayao0819/Kamisato/miko/domain"
	"github.com/Hayao0819/Kamisato/pkg/raiou"
)

type repositoryDBCall struct {
	repo string
	arch string
}

type fakeRepositoryDBReader struct {
	database *repo.RemoteRepo
	err      error
	calls    []repositoryDBCall
}

func (f *fakeRepositoryDBReader) Database(
	_ context.Context,
	repoName string,
	arch string,
) (*repo.RemoteRepo, error) {
	f.calls = append(f.calls, repositoryDBCall{repo: repoName, arch: arch})
	return f.database, f.err
}

// A monitored rebuild must enqueue a real job tagged ReasonVersionUpdate so its
// origin is visible, reusing Submit's validation.
func TestVersionUpdateEnqueuerTagsReason(t *testing.T) {
	s := New(Settings{})
	enq := &versionUpdateEnqueuer{s: s}

	entry := nvcheck.Entry{Pkgbase: "foo", Repo: "extra", Arch: "x86_64", Git: "https://aur.archlinux.org/foo.git"}
	if err := enq.EnqueueVersionUpdate(entry, "2.0.0"); err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	jobs := s.List()
	if len(jobs) != 1 {
		t.Fatalf("want 1 job, got %d", len(jobs))
	}
	if jobs[0].Reason != domain.ReasonVersionUpdate {
		t.Errorf("Reason = %q, want %q", jobs[0].Reason, domain.ReasonVersionUpdate)
	}
	if jobs[0].Arch != "x86_64" || jobs[0].Repo != "extra" {
		t.Errorf("job target = %s/%s, want extra/x86_64", jobs[0].Repo, jobs[0].Arch)
	}
}

func TestRepositoryConsumersShareInjectedReader(t *testing.T) {
	info := raiou.NewPKGINFO()
	info.PkgName = "foo-bin"
	info.PkgBase = "foo"
	info.PkgVer = "2.3.4-1"
	database := &repo.RemoteRepo{
		Name: "extra",
		Pkgs: []*ppkg.BinaryPackage{
			ppkg.NewBinaryPackage("foo-bin-2.3.4-1-x86_64.pkg.tar.zst", info),
		},
	}
	reader := &fakeRepositoryDBReader{database: database}
	s := New(Settings{AyatoURL: "https://ayato.example"}, WithRepositoryDBReader(reader))

	version, err := publishedVersion(s.repositories, true)(context.Background(), nvcheck.Entry{
		Pkgbase: "foo",
		Repo:    "extra",
		Arch:    "x86_64",
	})
	if err != nil {
		t.Fatal(err)
	}
	if version != "2.3.4-1" {
		t.Fatalf("version = %q", version)
	}
	gotDatabase, err := s.repositoryDB(context.Background(), "testing", "aarch64")
	if err != nil {
		t.Fatal(err)
	}
	if gotDatabase != database {
		t.Fatal("repositoryDB did not return the injected reader result")
	}
	wantCalls := []repositoryDBCall{
		{repo: "extra", arch: "x86_64"},
		{repo: "testing", arch: "aarch64"},
	}
	if len(reader.calls) != len(wantCalls) {
		t.Fatalf("calls = %#v, want %#v", reader.calls, wantCalls)
	}
	for i := range wantCalls {
		if reader.calls[i] != wantCalls[i] {
			t.Fatalf("calls[%d] = %#v, want %#v", i, reader.calls[i], wantCalls[i])
		}
	}
}

func TestPublishedVersionPreservesRepositoryFailure(t *testing.T) {
	wantErr := errors.New("repository unavailable")
	reader := &fakeRepositoryDBReader{err: wantErr}
	s := New(Settings{AyatoURL: "https://ayato.example"}, WithRepositoryDBReader(reader))

	_, err := publishedVersion(s.repositories, true)(context.Background(), nvcheck.Entry{
		Pkgbase: "foo",
		Repo:    "extra",
		Arch:    "x86_64",
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("error = %v, want %v", err, wantErr)
	}
}

func TestReadOnlyVersionCheckUsesHTTPAndRepositoryBoundaries(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		fmt.Fprint(response, "version=2.0")
	}))
	defer upstream.Close()
	info := raiou.NewPKGINFO()
	info.PkgName, info.PkgBase, info.PkgVer = "foo", "foo", "1.0-1"
	reader := &fakeRepositoryDBReader{database: &repo.RemoteRepo{Pkgs: []*ppkg.BinaryPackage{ppkg.NewBinaryPackage("foo-1.0-1-x86_64.pkg.tar.zst", info)}}}
	entries := []nvcheck.Entry{{Pkgbase: "foo", Repo: "extra", Arch: "x86_64", Source: nvcheck.Spec{Kind: "http", URL: upstream.URL, Regex: `version=(\S+)`}}}
	results := CheckUpstreamVersions(context.Background(), entries, upstream.Client(), reader)
	if len(results) != 1 || results[0].Err != nil || results[0].Latest != "2.0" || results[0].Current != "1.0-1" || !results[0].Outdated || results[0].Enqueued {
		t.Fatalf("read-only results = %+v", results)
	}
	if len(reader.calls) != 1 || reader.calls[0].repo != "extra" || reader.calls[0].arch != "x86_64" {
		t.Fatalf("reader calls = %+v", reader.calls)
	}
}
