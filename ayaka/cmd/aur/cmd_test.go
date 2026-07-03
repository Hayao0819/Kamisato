package aurcmd

import (
	"testing"
)

func TestAurAddRequiresAtLeastTwoArgs(t *testing.T) {
	cases := []struct {
		args []string
		desc string
	}{
		{[]string{"add"}, "no args"},
		{[]string{"add", "myrepo"}, "only srcrepo"},
	}
	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			cmd := Cmd()
			cmd.SetArgs(tc.args)
			if err := cmd.Execute(); err == nil {
				t.Error("expected error, got nil")
			}
		})
	}
}

func TestAurUpdateRequiresAtLeastTwoArgs(t *testing.T) {
	cases := []struct {
		args []string
		desc string
	}{
		{[]string{"update"}, "no args"},
		{[]string{"update", "myrepo"}, "only srcrepo"},
	}
	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			cmd := Cmd()
			cmd.SetArgs(tc.args)
			if err := cmd.Execute(); err == nil {
				t.Error("expected error, got nil")
			}
		})
	}
}

func TestAurAddInvalidRepoName(t *testing.T) {
	cmd := Cmd()
	cmd.SetArgs([]string{"add", "nonexistent-repo", "somepkg"})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for unknown repo name, got nil")
	}
}

func TestAurUpdateInvalidRepoName(t *testing.T) {
	cmd := Cmd()
	cmd.SetArgs([]string{"update", "nonexistent-repo", "somepkg"})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for unknown repo name, got nil")
	}
}
