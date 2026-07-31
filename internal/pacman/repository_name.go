package pacman

import "fmt"

func ValidateRepositoryName(name string) error {
	if name == "" {
		return fmt.Errorf("repository name is empty")
	}
	if name == "." || name == ".." {
		return fmt.Errorf("repository name %q is a path traversal component", name)
	}
	for _, r := range name {
		if isASCIIAlphaNumeric(r) || r == '.' || r == '_' || r == '-' {
			continue
		}
		return fmt.Errorf("repository name %q contains unsupported character %q", name, r)
	}
	return nil
}

func isASCIIAlphaNumeric(r rune) bool {
	return r >= 'a' && r <= 'z' ||
		r >= 'A' && r <= 'Z' ||
		r >= '0' && r <= '9'
}
