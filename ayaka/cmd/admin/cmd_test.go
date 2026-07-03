package admincmd

import (
	"testing"
)

func TestParseLoginOrID(t *testing.T) {
	cases := []struct {
		in        string
		wantID    int64
		wantLogin string
	}{
		{"42", 42, ""},
		{"0", 0, "0"},   // zero is not a valid ID -> treated as login
		{"-1", 0, "-1"}, // negative -> treated as login
		{"octocat", 0, "octocat"},
		{"some-user", 0, "some-user"},
	}
	for _, tc := range cases {
		id, login := parseLoginOrID(tc.in)
		if id != tc.wantID || login != tc.wantLogin {
			t.Errorf("parseLoginOrID(%q) = (%d, %q), want (%d, %q)",
				tc.in, id, login, tc.wantID, tc.wantLogin)
		}
	}
}

func TestAdminRemoveRequiresOneArg(t *testing.T) {
	cmd := Cmd()
	cmd.SetArgs([]string{"remove"})
	if err := cmd.Execute(); err == nil {
		t.Error("expected error for zero args to 'remove', got nil")
	}
}

func TestAdminAddRequiresOneArg(t *testing.T) {
	cmd := Cmd()
	cmd.SetArgs([]string{"add"})
	if err := cmd.Execute(); err == nil {
		t.Error("expected error for zero args to 'add', got nil")
	}
}
