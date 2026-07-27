package nvcheckcmd

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/Hayao0819/Kamisato/ayaka/app"
	"github.com/Hayao0819/Kamisato/pkg/nvcheck"
	"github.com/Hayao0819/Kamisato/pkg/pacman/repo"
)

type fakeNvChecker struct {
	results map[string][]nvcheck.Result
	calls   int
}

func (f *fakeNvChecker) RunNvCheck(_ context.Context, srcrepo *repo.SourceRepo, _ *http.Client) []nvcheck.Result {
	f.calls++
	return f.results[srcrepo.Config.Name]
}

func testApp() *app.App {
	return &app.App{SrcRepos: []*repo.SourceRepo{
		{Config: &repo.SrcConfig{Name: "alpha"}},
		{Config: &repo.SrcConfig{Name: "beta"}},
	}}
}

func TestNvcheckWalksEveryRepo(t *testing.T) {
	fake := &fakeNvChecker{results: map[string][]nvcheck.Result{
		"alpha": {{Pkgbase: "foo", Current: "1.0", Latest: "1.0"}},
		"beta":  {{Pkgbase: "bar", Current: "1.0", Latest: "1.0"}},
	}}
	cmd := newCommand(fake)
	cmd.SetContext(app.WithContext(t.Context(), testApp()))
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if fake.calls != 2 {
		t.Errorf("RunNvCheck calls = %d, want 2", fake.calls)
	}
	for _, want := range []string{"alpha", "foo", "beta", "bar", "up-to-date"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("output misses %q: %q", want, out.String())
		}
	}
}

func TestNvcheckOutdatedExitsNonZero(t *testing.T) {
	fake := &fakeNvChecker{results: map[string][]nvcheck.Result{
		"alpha": {{Pkgbase: "foo", Current: "1.0", Latest: "2.0", Outdated: true}},
	}}
	cmd := newCommand(fake)
	cmd.SetContext(app.WithContext(t.Context(), testApp()))
	cmd.SetOut(&bytes.Buffer{})
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	if err := cmd.Execute(); err == nil {
		t.Error("outdated package should exit non-zero")
	}
}

func TestNvcheckJSONRows(t *testing.T) {
	fake := &fakeNvChecker{results: map[string][]nvcheck.Result{
		"alpha": {
			{Pkgbase: "foo", Current: "1.0", Latest: "2.0", Outdated: true},
			{Pkgbase: "bad", Err: errors.New("boom")},
		},
	}}
	cmd := newCommand(fake)
	cmd.SetContext(app.WithContext(t.Context(), testApp()))
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--format", "json"})
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	_ = cmd.Execute()

	for _, want := range []string{
		`"repo":"alpha"`, `"pkgbase":"foo"`, `"status":"OUTDATED"`, `"status":"error: boom"`,
	} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("json output misses %s: %q", want, out.String())
		}
	}
}
