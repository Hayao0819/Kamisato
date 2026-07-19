package reponame

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
		if err := Validate(name); err != nil {
			t.Errorf("Validate(%q) = %v, want nil", name, err)
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
		if err := Validate(name); err == nil {
			t.Errorf("Validate(%q) = nil, want error", name)
		}
	}
}
