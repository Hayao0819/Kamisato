package buildcmd

import (
	"strings"
	"testing"

	"github.com/Hayao0819/Kamisato/ayaka/app"
	"github.com/Hayao0819/Kamisato/pkg/pacman/repo"
)

func TestBuildFlagShape(t *testing.T) {
	cmd := Cmd()
	flags := cmd.Flags()

	present := []string{"sign", "key", "diff", "server", "executor", "arch", "publish", "publish-url", "publish-server"}
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
	// --diff must not exempt the key check, or --sign --diff silently builds
	// unsigned packages.
	for _, args := range [][]string{
		{"--sign", "extra"},
		{"--sign", "--diff", "extra"},
	} {
		cmd := Cmd()
		a := &app.App{SrcRepos: []*repo.SourceRepo{
			{Config: &repo.SrcConfig{Name: "extra"}},
		}}
		cmd.SetContext(app.WithContext(t.Context(), a))
		cmd.SetArgs(args)
		cmd.SilenceErrors = true
		cmd.SilenceUsage = true

		err := cmd.Execute()
		if err == nil {
			t.Fatalf("%v without --key succeeded", args)
		}
		if !strings.Contains(err.Error(), "--sign requires --key") {
			t.Fatalf("%v: error = %q, want missing --key error", args, err)
		}
	}
}

func TestBuildUseString(t *testing.T) {
	cmd := Cmd()
	if !strings.HasPrefix(cmd.Use, "build <srcrepo>") {
		t.Errorf("Use = %q, want prefix 'build <srcrepo>'", cmd.Use)
	}
}
