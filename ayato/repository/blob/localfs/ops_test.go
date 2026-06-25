package localfs

import "testing"

// TestValidatePathComponentRejectsTraversal locks the guard that keeps an
// attacker-controlled package arch / filename from escaping the repo directory.
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
