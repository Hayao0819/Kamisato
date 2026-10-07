package service

import (
	"context"
	"errors"
	"testing"

	ppkg "github.com/Hayao0819/Kamisato/internal/pacman/pkg"
	"github.com/Hayao0819/Kamisato/internal/pacman/repo"
	"github.com/Hayao0819/Kamisato/internal/pacman/source/nvcheck"
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

	version, err := s.publishedVersion()(context.Background(), nvcheck.Entry{
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

	_, err := s.publishedVersion()(context.Background(), nvcheck.Entry{
		Pkgbase: "foo",
		Repo:    "extra",
		Arch:    "x86_64",
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("error = %v, want %v", err, wantErr)
	}
}
