package cmd

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
)

func TestEveryCommandHelpIsIndependentOfConfigurationAndCredentials(t *testing.T) {
	t.Setenv("AYAKA_BUILDER_TIMEOUT", "not-a-duration")
	root := RootCmd()
	var paths [][]string
	var collect func([]string)
	collect = func(path []string) {
		command, _, err := root.Find(path)
		if err != nil {
			t.Fatal(err)
		}
		paths = append(paths, append([]string(nil), path...))
		for _, child := range command.Commands() {
			if !child.Hidden {
				collect(append(append([]string(nil), path...), child.Name()))
			}
		}
	}
	collect(nil)
	for _, path := range paths {
		t.Run(strings.Join(path, "/"), func(t *testing.T) {
			command := RootCmd()
			args := append(append([]string(nil), path...), "--config", filepath.Join(t.TempDir(), "missing.json"), "--help")
			command.SetArgs(args)
			var output bytes.Buffer
			command.SetOut(&output)
			command.SetErr(&output)
			if err := command.Execute(); err != nil {
				t.Fatalf("help loaded external input: %v", err)
			}
			if output.Len() == 0 {
				t.Fatal("help output is empty")
			}
		})
	}
}

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
