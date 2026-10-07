package buildset

import (
	"fmt"
	"slices"
	"strings"
)

func normalizePackages(names []string) ([]string, error) {
	seen := make(map[string]struct{}, len(names))
	packages := make([]string, 0, len(names))
	for _, name := range names {
		if err := validatePackageName(name); err != nil {
			return nil, err
		}
		if _, exists := seen[name]; exists {
			continue
		}
		seen[name] = struct{}{}
		packages = append(packages, name)
	}
	slices.Sort(packages)
	return packages, nil
}

func validatePackageName(name string) error {
	if name == "" || name == "." || name == ".." || name[0] == '-' || name[0] == '.' {
		return fmt.Errorf("invalid package name %q", name)
	}
	for _, r := range name {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || strings.ContainsRune("@._+-", r) {
			continue
		}
		return fmt.Errorf("invalid package name %q", name)
	}
	return nil
}

func validateTargetArchitecture(arch string) error {
	switch arch {
	case "x86_64", "i486", "i686", "pentium4", "aarch64", "armv7h":
		return nil
	case "":
		return fmt.Errorf("target architecture is required")
	default:
		return fmt.Errorf("unsupported target architecture %q", arch)
	}
}
