package buildcmd

import (
	"strings"
	"testing"

	"github.com/Hayao0819/Kamisato/ayaka/app"
	"github.com/Hayao0819/Kamisato/internal/pacman/source"
)

func TestBuildFlagShape(t *testing.T) {
	cmd := Cmd(app.StaticRuntime(&app.App{}))
	flags := cmd.Flags()

	present := []string{
		"source-repo", "local-source", "pacman-conf", "repo", "output", "manifest", "backend", "image", "work-dir", "keep-work", "log-dir",
		"makepkg-option", "sign", "key", "diff", "server", "executor", "arch", "publish", "publish-url", "publish-server",
	}
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
		{"--source-repo", "extra", "--sign"},
		{"--source-repo", "extra", "--sign", "--diff"},
	} {
		a := &app.App{SrcRepos: []*source.SourceRepo{
			{Config: &source.SrcConfig{Name: "extra"}},
		}}
		cmd := Cmd(app.StaticRuntime(a))
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
	cmd := Cmd(app.StaticRuntime(&app.App{}))
	if cmd.Use != "build [pkgname...]" {
		t.Errorf("Use = %q", cmd.Use)
	}
}

func TestBuildRejectsDirectInputsWithSourceRepo(t *testing.T) {
	a := &app.App{SrcRepos: []*source.SourceRepo{{Config: &source.SrcConfig{Name: "extra"}}}}
	cmd := Cmd(app.StaticRuntime(a))
	cmd.SetArgs([]string{"--source-repo", "extra", "--local-source", "./pkg"})
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "--local-source cannot be used") {
		t.Fatalf("error = %v", err)
	}
}

func TestBuildRejectsMissingDirectOutput(t *testing.T) {
	cmd := Cmd(app.StaticRuntime(&app.App{}))
	cmd.SetArgs([]string{"example", "--pacman-conf", "pacman.conf"})
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "--output and --manifest") {
		t.Fatalf("error = %v", err)
	}
}

func TestBuildRejectsAlterisoListFlags(t *testing.T) {
	cmd := Cmd(app.StaticRuntime(&app.App{}))
	for _, name := range []string{"aur-list", "pkgbuild-root"} {
		if cmd.Flags().Lookup(name) != nil {
			t.Errorf("flag --%s should not be registered", name)
		}
	}
}
