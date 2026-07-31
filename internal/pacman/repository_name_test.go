package pacman

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
		if err := ValidateRepositoryName(name); err != nil {
			t.Errorf("ValidateRepositoryName(%q) = %v, want nil", name, err)
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
		if err := ValidateRepositoryName(name); err == nil {
			t.Errorf("ValidateRepositoryName(%q) = nil, want error", name)
		}
	}
}
