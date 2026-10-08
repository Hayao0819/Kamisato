package addcmd

import (
	"context"
	"testing"

	"github.com/Hayao0819/Kamisato/ayaka/cmd/internal/sourcerepos"
	sourcesvc "github.com/Hayao0819/Kamisato/ayaka/service/source"
	"github.com/Hayao0819/Kamisato/ayaka/source"
)

func TestAurCommandsRejectInvalidArguments(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{"add without arguments", []string{}},
		{"add without packages", []string{"myrepo"}},
		{"add to unknown repository", []string{"nonexistent-repo", "somepkg"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cmd := newCommand(sourcesvc.AddAUR, sourcerepos.Static(nil))
			cmd.SetArgs(tc.args)
			if err := cmd.Execute(); err == nil {
				t.Error("expected error, got nil")
			}
		})
	}
}

type recordingAurManager struct {
	addDir   string
	addNames []string
	addForce bool
}

func (r *recordingAurManager) Add(_ context.Context, dir string, names []string, force bool) error {
	r.addDir, r.addNames, r.addForce = dir, names, force
	return nil
}

func testSources(t *testing.T) []*source.SourceRepo {
	t.Helper()
	return []*source.SourceRepo{{
		Config: &source.SrcConfig{Name: "test"},
		Dir:    "/src/test",
	}}
}

func TestAurAddFlagsReachService(t *testing.T) {
	rec := &recordingAurManager{}
	cmd := newCommand(rec.Add, sourcerepos.Static(testSources(t)))
	cmd.SetArgs([]string{"test", "yay", "yay-bin", "--force"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if rec.addDir != "/src/test" || len(rec.addNames) != 2 || !rec.addForce {
		t.Errorf("service got dir=%q names=%v force=%v", rec.addDir, rec.addNames, rec.addForce)
	}
}

func TestAurNoSourceDirFails(t *testing.T) {
	a := []*source.SourceRepo{{Config: &source.SrcConfig{Name: "test"}}}
	cmd := newCommand((&recordingAurManager{}).Add, sourcerepos.Static(a))
	cmd.SetArgs([]string{"test", "yay"})
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	if err := cmd.Execute(); err == nil {
		t.Error("source repo with no Dir should error")
	}
}
