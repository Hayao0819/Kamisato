package cmd

import (
	"os"
	"testing"
)

func TestRealMakepkgUsesResolvedConfiguration(t *testing.T) {
	t.Setenv("THOMA_MAKEPKG", "must-not-override-explicit-settings")
	if got := realMakepkg("/explicit/makepkg"); got != "/explicit/makepkg" {
		t.Fatalf("makepkg = %q, want explicit configured executable", got)
	}
}

func TestRealMakepkgSkipsShimExecutable(t *testing.T) {
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	if got := realMakepkg(self); sameFile(got, self) {
		t.Fatalf("makepkg resolves recursively to shim: %q", got)
	}
}
