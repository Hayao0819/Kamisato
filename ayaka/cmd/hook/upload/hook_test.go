package uploadcmd

import (
	"strings"
	"testing"
)

func TestUploadRepoRequired(t *testing.T) {
	cmd := Cmd()
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	// Execute with no flags: cobra must reject before RunE because --repo is MarkFlagRequired.
	cmd.SetArgs([]string{})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when --repo is not provided")
	}
	if !strings.Contains(err.Error(), "repo") {
		t.Errorf("error %q does not mention 'repo'", err.Error())
	}
}

func TestUploadFlagShape(t *testing.T) {
	cmd := Cmd()
	flags := cmd.Flags()

	present := []string{"repo", "pacman-config", "cache-dir", "build-dir", "all", "timeout"}
	for _, name := range present {
		if flags.Lookup(name) == nil {
			t.Errorf("flag --%s not registered", name)
		}
	}
}
