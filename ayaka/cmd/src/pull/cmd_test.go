package pullcmd

import (
	"context"
	"testing"

	"github.com/Hayao0819/Kamisato/ayaka/app"
	pkg "github.com/Hayao0819/Kamisato/pkg/pacman/pkg"
	"github.com/Hayao0819/Kamisato/pkg/pacman/repo"
)

type recordingPuller struct {
	names []string
	force bool
	calls int
}

func (r *recordingPuller) Pull(_ context.Context, src *repo.SourceRepo, names []string, force bool) ([]*pkg.SourcePackage, error) {
	r.names = names
	r.force = force
	r.calls++
	return src.Pkgs, nil
}

func testApp() *app.App {
	return &app.App{SrcRepos: []*repo.SourceRepo{{Config: &repo.SrcConfig{Name: "test"}}}}
}

func TestPullFlagsReachService(t *testing.T) {
	rec := &recordingPuller{}
	cmd := newCommand(rec)
	cmd.SetContext(app.WithContext(t.Context(), testApp()))
	cmd.SetArgs([]string{"test", "ckbcomp", "foo", "--force"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if len(rec.names) != 2 || rec.names[0] != "ckbcomp" || !rec.force {
		t.Errorf("service got names=%v force=%v", rec.names, rec.force)
	}
}

func TestPullAllMirrorsWithNoNames(t *testing.T) {
	rec := &recordingPuller{}
	cmd := newCommand(rec)
	cmd.SetContext(app.WithContext(t.Context(), testApp()))
	cmd.SetArgs([]string{"test"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if rec.calls != 1 || len(rec.names) != 0 {
		t.Errorf("calls=%d names=%v, want one call with no names", rec.calls, rec.names)
	}
}

func TestPullUnknownRepoFails(t *testing.T) {
	cmd := newCommand(&recordingPuller{})
	cmd.SetContext(app.WithContext(t.Context(), testApp()))
	cmd.SetArgs([]string{"nope"})
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	if err := cmd.Execute(); err == nil {
		t.Error("unknown repo should error")
	}
}
