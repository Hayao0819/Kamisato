package localfs

import "testing"

// Locks the guard against attacker-controlled arch / filename escaping the repo dir.
func TestValidatePathComponentRejectsTraversal(t *testing.T) {
	bad := []string{"", ".", "..", "../x", "a/b", "x/../y", "/abs", "etc/passwd"}
	for _, c := range bad {
		if err := validatePathComponent(c); err == nil {
			t.Fatalf("validatePathComponent(%q) = nil, want error", c)
		}
	}
	good := []string{"x86_64", "any", "aarch64", "pkg-1.0-1-x86_64.pkg.tar.zst"}
	for _, c := range good {
		if err := validatePathComponent(c); err != nil {
			t.Fatalf("validatePathComponent(%q) = %v, want nil", c, err)
		}
	}
}
