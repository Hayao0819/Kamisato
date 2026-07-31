package builder

import "fmt"

var microarchLevels = map[string]string{
	"x86_64_v2": "x86-64-v2",
	"x86_64_v3": "x86-64-v3",
	"x86_64_v4": "x86-64-v4",
}

// ValidMicroarch reports whether tier is empty or a supported x86-64 feature level.
func ValidMicroarch(tier string) bool {
	if tier == "" {
		return true
	}
	_, ok := microarchLevels[tier]
	return ok
}

// MicroarchMarch resolves an x86-64 feature-level name to its compiler -march
// value. The empty tier returns an empty value.
func MicroarchMarch(tier string) (string, error) {
	if tier == "" {
		return "", nil
	}
	march, ok := microarchLevels[tier]
	if !ok {
		return "", fmt.Errorf("unknown microarchitecture tier %q", tier)
	}
	return march, nil
}
