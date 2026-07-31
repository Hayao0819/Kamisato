package cmd

import (
	"bytes"
	"strings"
	"testing"
)

func TestVersionDoesNotLoadAyakaConfig(t *testing.T) {
	t.Setenv("AYAKA_BUILDER_TIMEOUT", "not-a-duration")
	command := RootCmd()
	command.SetArgs([]string{"version"})
	var output bytes.Buffer
	command.SetOut(&output)
	command.SetErr(&output)
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "ayaka version") {
		t.Fatalf("version output = %q", output.String())
	}
}
