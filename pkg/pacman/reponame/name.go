// Package reponame defines the repository-name grammar shared by clients,
// services, and storage backends.
package reponame

import "fmt"

// Validate rejects names that cannot safely be used as a pacman repository
// section name, URL segment, object-key component, and filesystem directory.
//
// The grammar intentionally matches the characters historically accepted by
// miko. Dots may occur inside a name, but the traversal components "." and ".."
// are never repository names.
func Validate(name string) error {
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

// IsValid reports whether name satisfies Validate.
func IsValid(name string) bool {
	return Validate(name) == nil
}

func isASCIIAlphaNumeric(r rune) bool {
	return r >= 'a' && r <= 'z' ||
		r >= 'A' && r <= 'Z' ||
		r >= '0' && r <= '9'
}
