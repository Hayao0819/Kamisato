//go:build integration

package pacman

import (
	"os/exec"
	"testing"
)

func TestGetCleanPkgBinary(t *testing.T) {
	for _, bin := range []string{"pacman", "fakeroot"} {
		if _, err := exec.LookPath(bin); err != nil {
			t.Skipf("%s not installed; skipping integration download test", bin)
		}
	}

	files, cleanup, err := GetCleanPkgBinary("git")
	if err != nil {
		t.Fatalf("GetCleanPkgBinary failed: %v", err)
	}
	defer func() { _ = cleanup.Close() }()
	if len(files) == 0 {
		t.Fatal("GetCleanPkgBinary returned no package files")
	}
}
