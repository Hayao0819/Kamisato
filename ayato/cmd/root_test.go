package cmd

import (
	"io"
	"path/filepath"
	"strings"
	"testing"
)

func TestHelpAndVersionDoNotLoadConfiguration(t *testing.T) {
	for _, path := range []string{"", "aur", "aur keygen", "kv", "kv audit", "repo", "repo gc", "migrate", "version"} {
		t.Run(path, func(t *testing.T) {
			command := RootCmd()
			command.SetOut(io.Discard)
			command.SetErr(io.Discard)
			args := []string{"--config", filepath.Join(t.TempDir(), "nonexistent.json")}
			args = append(args, strings.Fields(path)...)
			if path != "version" {
				args = append(args, "--help")
			}
			command.SetArgs(args)
			if err := command.Execute(); err != nil {
				t.Fatalf("%s accessed unrelated configuration: %v", path, err)
			}
		})
	}
}
