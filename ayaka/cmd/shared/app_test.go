package shared

import (
	"context"
	"testing"

	"github.com/Hayao0819/Kamisato/internal/conf"
	"github.com/Hayao0819/Kamisato/pkg/pacman/repo"
)

func testApp() *App {
	return &App{
		Config: &conf.AyakaConfig{},
		SrcRepos: []*repo.SourceRepo{
			{Config: &repo.SrcConfig{Name: "extra"}, Dir: "/src/extra", DestDir: "/out/extra"},
			{Config: &repo.SrcConfig{Name: "community"}, Dir: "/src/community", DestDir: "/out/community"},
		},
	}
}

func TestAppLookups(t *testing.T) {
	app := testApp()

	if r := app.GetSrcRepo("community"); r == nil || r.Dir != "/src/community" {
		t.Errorf("GetSrcRepo(community) = %v, want the community repo", r)
	}
	if r := app.GetSrcRepo("missing"); r != nil {
		t.Errorf("GetSrcRepo(missing) = %v, want nil", r)
	}
	if got := app.GetSrcDir("extra"); got != "/src/extra" {
		t.Errorf("GetSrcDir(extra) = %q, want /src/extra", got)
	}
	if got := app.GetDestDir("extra"); got != "/out/extra" {
		t.Errorf("GetDestDir(extra) = %q, want /out/extra", got)
	}
	if got := app.GetSrcDir("missing"); got != "" {
		t.Errorf("GetSrcDir(missing) = %q, want empty", got)
	}
	names := app.GetSrcRepoNames()
	if len(names) != 2 || names[0] != "extra" || names[1] != "community" {
		t.Errorf("GetSrcRepoNames = %v, want [extra community]", names)
	}
}

func TestAppFromContextRoundTrip(t *testing.T) {
	app := testApp()
	ctx := WithApp(context.Background(), app)
	if got := AppFromContext(ctx); got != app {
		t.Errorf("AppFromContext did not return the stored App")
	}
}

// AppFromContext must never return nil, so completion (which runs before the
// App is built) can call methods without a nil-pointer panic.
func TestAppFromContextEmptyWhenUnset(t *testing.T) {
	got := AppFromContext(context.Background())
	if got == nil {
		t.Fatal("AppFromContext(empty) = nil, want a non-nil empty App")
	}
	if names := got.GetSrcRepoNames(); len(names) != 0 {
		t.Errorf("empty App GetSrcRepoNames = %v, want none", names)
	}
	if r := got.GetSrcRepo("anything"); r != nil {
		t.Errorf("empty App GetSrcRepo = %v, want nil", r)
	}
}
