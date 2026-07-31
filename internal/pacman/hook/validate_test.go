package hook

import "testing"

func TestValidateExecArg(t *testing.T) {
	ok := []string{"", "myrepo", "/etc/pacman.conf", "server-1", "a.b_c"}
	for _, v := range ok {
		if err := ValidateExecArg("--x", v); err != nil {
			t.Errorf("ValidateExecArg(%q) = %v, want nil", v, err)
		}
	}
	bad := []string{"x --all", "a\tb", "x\ny", `x"y`, "x'y", `x\y`}
	for _, v := range bad {
		if err := ValidateExecArg("--x", v); err == nil {
			t.Errorf("ValidateExecArg(%q) = nil, want error (would re-tokenize the Exec line)", v)
		}
	}
}
