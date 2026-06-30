package alpm

import (
	"os/exec"
	"testing"
)

// TestGetCleanPkgBinary is an integration test: it downloads from a real pacman
// mirror via fakeroot+pacman, so it skips under -short and when the required
// tooling is absent (non-Arch hosts, CI without pacman).
func TestGetCleanPkgBinary(t *testing.T) {
	if testing.Short() {
		t.Skip("downloads from a pacman mirror; skipped in -short")
	}
	for _, bin := range []string{"pacman", "fakeroot"} {
		if _, err := exec.LookPath(bin); err != nil {
			t.Skipf("%s not installed; skipping integration download test", bin)
		}
	}

	_, err := GetCleanPkgBinary("git")
	if err != nil {
		t.Fatalf("GetCleanPkgBinary failed: %v", err)
	}
}
