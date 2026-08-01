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

func TestRootHasBuildAndPlanWithoutBuildSet(t *testing.T) {
	root := RootCmd()
	for _, name := range []string{"build", "plan"} {
		command, _, err := root.Find([]string{name})
		if err != nil || command == root || command.Name() != name {
			t.Errorf("command %q not found: command=%v error=%v", name, command, err)
		}
	}
	command, _, err := root.Find([]string{"build-set"})
	if err == nil && command != root && command.Name() == "build-set" {
		t.Fatal("build-set command should not be registered")
	}
}
