package statuscmd

import (
	"testing"

	"github.com/Hayao0819/Kamisato/ayaka/app"
	"github.com/Hayao0819/Kamisato/pkg/pacman/repo"
)

func TestStatusArgsValidation(t *testing.T) {
	cmd := Cmd()
	cmd.SetArgs([]string{"repo1", "repo2"})
	if err := cmd.Execute(); err == nil {
		t.Error("expected error for two positional args, got nil")
	}
}

func TestStatusUnknownRepoFails(t *testing.T) {
	cmd := Cmd()
	a := &app.App{SrcRepos: []*repo.SourceRepo{{Config: &repo.SrcConfig{Name: "test"}}}}
	cmd.SetContext(app.WithContext(t.Context(), a))
	cmd.SetArgs([]string{"nope"})
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	if err := cmd.Execute(); err == nil {
		t.Error("unknown repo should error")
	}
}
