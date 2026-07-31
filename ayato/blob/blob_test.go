package blob

import "testing"

// Attacker-controlled repo/arch/filename must not escape the directory or key prefix.
func TestValidatePathComponentRejectsTraversal(t *testing.T) {
	bad := []string{"", ".", "..", "../x", "a/b", "x/../y", "/abs", "etc/passwd"}
	for _, c := range bad {
		if err := ValidatePathComponent(c); err == nil {
			t.Fatalf("ValidatePathComponent(%q) = nil, want error", c)
		}
	}
	good := []string{"x86_64", "any", "aarch64", "pkg-1.0-1-x86_64.pkg.tar.zst"}
	for _, c := range good {
		if err := ValidatePathComponent(c); err != nil {
			t.Fatalf("ValidatePathComponent(%q) = %v, want nil", c, err)
		}
	}
}
