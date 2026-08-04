package cli

import (
	"errors"
	"slices"
	"testing"

	"github.com/spf13/cobra"

	"github.com/Hayao0819/Kamisato/internal/cliutil"
)

func TestDirectBuildFlagsNormalizeCLIInput(t *testing.T) {
	command := &cobra.Command{Use: "build"}
	var flags DirectBuildFlags
	flags.Add(command)
	if err := command.ParseFlags([]string{
		"--local-source", "./one", "--local-source", "./two", "--pacman-conf", "pacman.conf", "--arch", "i486",
		"--makepkg-option", "!debug",
	}); err != nil {
		t.Fatal(err)
	}
	request := flags.PlanRequest([]string{"foo", "bar"})
	if !slices.Equal(request.Packages, []string{"foo", "bar"}) || !slices.Equal(request.LocalSources, []string{"./one", "./two"}) {
		t.Fatalf("request = %+v", request)
	}
	if request.Arch != "i486" || request.PacmanConf != "pacman.conf" {
		t.Fatalf("request = %+v", request)
	}
	options := flags.ApplicationOptions("")
	if !slices.Equal(options.Makepkg.Options, []string{"!debug"}) {
		t.Fatalf("makepkg options = %v", options.Makepkg.Options)
	}
}

func TestDirectBuildFlagsRejectMissingInputsInCLILayer(t *testing.T) {
	var flags DirectBuildFlags
	err := flags.Validate(nil)
	var usage *cliutil.UsageError
	if !errors.As(err, &usage) {
		t.Fatalf("error = %v", err)
	}
}
