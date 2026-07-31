package statuscmd

import (
	"testing"

	"github.com/Hayao0819/Kamisato/ayaka/app"
	"github.com/Hayao0819/Kamisato/internal/pacman/source"
)

func TestStatusArgsValidation(t *testing.T) {
	cmd := Cmd(app.StaticRuntime(&app.App{}))
	cmd.SetArgs([]string{"repo1", "repo2"})
	if err := cmd.Execute(); err == nil {
		t.Error("expected error for two positional args, got nil")
	}
}

func TestStatusUnknownRepoFails(t *testing.T) {
	a := &app.App{SrcRepos: []*source.SourceRepo{{Config: &source.SrcConfig{Name: "test"}}}}
	cmd := Cmd(app.StaticRuntime(a))
	cmd.SetArgs([]string{"nope"})
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	if err := cmd.Execute(); err == nil {
		t.Error("unknown repo should error")
	}
}
