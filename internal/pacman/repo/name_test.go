package repo

import "testing"

func TestValidate(t *testing.T) {
	t.Parallel()

	for _, name := range []string{
		"core",
		"core-staging",
		"custom_repo",
		"repo.v2",
		".private",
		"repo..snapshot",
	} {
		if err := ValidateName(name); err != nil {
			t.Errorf("ValidateName(%q) = %v, want nil", name, err)
		}
	}

	for _, name := range []string{
		"",
		".",
		"..",
		"../core",
		"core/testing",
		`core\testing`,
		"core\nextra",
		"core]",
		"日本語",
	} {
		if err := ValidateName(name); err == nil {
			t.Errorf("ValidateName(%q) = nil, want error", name)
		}
	}
}
