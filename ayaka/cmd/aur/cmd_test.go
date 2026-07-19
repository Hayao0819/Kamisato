package aurcmd

import (
	"testing"
)

func TestAurCommandsRejectInvalidArguments(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{"add without arguments", []string{"add"}},
		{"add without packages", []string{"add", "myrepo"}},
		{"add to unknown repository", []string{"add", "nonexistent-repo", "somepkg"}},
		{"update without arguments", []string{"update"}},
		{"update without packages", []string{"update", "myrepo"}},
		{"update in unknown repository", []string{"update", "nonexistent-repo", "somepkg"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cmd := Cmd()
			cmd.SetArgs(tc.args)
			if err := cmd.Execute(); err == nil {
				t.Error("expected error, got nil")
			}
		})
	}
}
