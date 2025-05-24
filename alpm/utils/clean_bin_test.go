package pkg

import (
	"testing"
)

func TestGetCleanPkgBinary(t *testing.T) {
	_, err := GetCleanPkgBinary("git")
	if err != nil {
		t.Fatalf("GetCleanPkgBinary failed: %v", err)
	}
}
