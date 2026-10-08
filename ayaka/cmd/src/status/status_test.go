package statuscmd

import (
	"testing"

	"github.com/Hayao0819/Kamisato/ayaka/cmd/internal/sourcerepos"
	"github.com/Hayao0819/Kamisato/ayaka/source"
)

func TestStatusArgsValidation(t *testing.T) {
	cmd := newCommand(sourcerepos.Static(nil))
	cmd.SetArgs([]string{"repo1", "repo2"})
	if err := cmd.Execute(); err == nil {
		t.Error("expected error for two positional args, got nil")
	}
}

func TestStatusUnknownRepoFails(t *testing.T) {
	a := []*source.SourceRepo{{Config: &source.SrcConfig{Name: "test"}}}
	cmd := newCommand(sourcerepos.Static(a))
	cmd.SetArgs([]string{"nope"})
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	if err := cmd.Execute(); err == nil {
		t.Error("unknown repo should error")
	}
}
