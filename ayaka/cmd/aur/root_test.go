package aurcmd

import "testing"

func TestValidateAurPkgName(t *testing.T) {
	valid := []string{"yay", "yay-bin", "lib32-glibc", "python3.11", "g++", "foo_bar", "0ad", "a.b.c"}
	for _, name := range valid {
		if err := validateAurPkgName(name); err != nil {
			t.Errorf("validateAurPkgName(%q) = %v, want nil", name, err)
		}
	}

	invalid := []string{
		"",
		"..",
		"../x",
		"../../etc/passwd",
		"foo/bar",
		"/abs",
		".hidden",
		"-leading-dash",
		"Upper",
		"with space",
		"semi;colon",
		"new\nline",
	}
	for _, name := range invalid {
		if err := validateAurPkgName(name); err == nil {
			t.Errorf("validateAurPkgName(%q) = nil, want error", name)
		}
	}
}
