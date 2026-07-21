package buildcmd

import (
	"context"
	"strings"
	"testing"

	"github.com/Hayao0819/Kamisato/ayaka/cmd/shared"
	"github.com/Hayao0819/Kamisato/pkg/pacman/repo"
)

func TestBuildFlagShape(t *testing.T) {
	cmd := Cmd()
	flags := cmd.Flags()

	present := []string{"sign", "key", "diff", "server", "executor", "arch"}
	for _, name := range present {
		if flags.Lookup(name) == nil {
			t.Errorf("flag --%s not registered", name)
		}
	}

	absent := []string{"gpgkey", "remote"}
	for _, name := range absent {
		if flags.Lookup(name) != nil {
			t.Errorf("flag --%s should have been removed", name)
		}
	}

	// --key must not have a shorthand.
	if f := flags.Lookup("key"); f != nil {
		if sh := flags.ShorthandLookup("g"); sh != nil {
			t.Errorf("shorthand -g should no longer exist (was the old --key shorthand)")
		}
	}
}

func TestBuildSignRequiresKey(t *testing.T) {
	cmd := Cmd()
	app := &shared.App{SrcRepos: []*repo.SourceRepo{
		{Config: &repo.SrcConfig{Name: "extra"}},
	}}
	cmd.SetContext(shared.WithApp(context.Background(), app))
	cmd.SetArgs([]string{"--sign", "extra"})
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true

	err := cmd.Execute()
	if err == nil {
		t.Fatal("--sign without --key succeeded")
	}
	if !strings.Contains(err.Error(), "--sign requires --key") {
		t.Fatalf("error = %q, want missing --key error", err)
	}
}

func TestBuildUseString(t *testing.T) {
	cmd := Cmd()
	if !strings.HasPrefix(cmd.Use, "build <srcrepo>") {
		t.Errorf("Use = %q, want prefix 'build <srcrepo>'", cmd.Use)
	}
}
