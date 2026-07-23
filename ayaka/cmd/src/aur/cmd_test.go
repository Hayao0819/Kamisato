package aurcmd

import (
	"context"
	"testing"

	"github.com/Hayao0819/Kamisato/ayaka/app"
	"github.com/Hayao0819/Kamisato/pkg/pacman/repo"
)

func TestAurCommandsRejectInvalidArguments(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{"add without arguments", []string{"add"}},
		{"add without packages", []string{"add", "myrepo"}},
		{"add to unknown repository", []string{"add", "nonexistent-repo", "somepkg"}},
		{"update without arguments", []string{"update"}},
		{"update without packages", []string{"update", "myrepo"}},
		{"update in unknown repository", []string{"update", "nonexistent-repo", "somepkg"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cmd := Cmd()
			cmd.SetArgs(tc.args)
			if err := cmd.Execute(); err == nil {
				t.Error("expected error, got nil")
			}
		})
	}
}

type recordingAurManager struct {
	addDir, updateDir     string
	addNames, updateNames []string
	addForce, updateForce bool
}

func (r *recordingAurManager) Add(_ context.Context, dir string, names []string, force bool) error {
	r.addDir, r.addNames, r.addForce = dir, names, force
	return nil
}

func (r *recordingAurManager) Update(_ context.Context, dir string, names []string, force bool) error {
	r.updateDir, r.updateNames, r.updateForce = dir, names, force
	return nil
}

func testApp(t *testing.T) *app.App {
	t.Helper()
	return &app.App{SrcRepos: []*repo.SourceRepo{{
		Config: &repo.SrcConfig{Name: "test"},
		Dir:    "/src/test",
	}}}
}

func TestAurAddFlagsReachService(t *testing.T) {
	rec := &recordingAurManager{}
	cmd := newCommand(rec)
	cmd.SetContext(app.WithContext(t.Context(), testApp(t)))
	cmd.SetArgs([]string{"add", "test", "yay", "yay-bin", "--force"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if rec.addDir != "/src/test" || len(rec.addNames) != 2 || !rec.addForce {
		t.Errorf("service got dir=%q names=%v force=%v", rec.addDir, rec.addNames, rec.addForce)
	}
}

func TestAurUpdateFlagsReachService(t *testing.T) {
	rec := &recordingAurManager{}
	cmd := newCommand(rec)
	cmd.SetContext(app.WithContext(t.Context(), testApp(t)))
	cmd.SetArgs([]string{"update", "test", "yay"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if rec.updateDir != "/src/test" || len(rec.updateNames) != 1 || rec.updateNames[0] != "yay" || rec.updateForce {
		t.Errorf("service got dir=%q names=%v force=%v", rec.updateDir, rec.updateNames, rec.updateForce)
	}
}

func TestAurNoSourceDirFails(t *testing.T) {
	cmd := newCommand(&recordingAurManager{})
	a := &app.App{SrcRepos: []*repo.SourceRepo{{Config: &repo.SrcConfig{Name: "test"}}}}
	cmd.SetContext(app.WithContext(t.Context(), a))
	cmd.SetArgs([]string{"add", "test", "yay"})
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	if err := cmd.Execute(); err == nil {
		t.Error("source repo with no Dir should error")
	}
}
