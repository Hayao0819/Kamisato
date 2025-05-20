package remote_test

import (
	"testing"

	"github.com/Hayao0819/Kamisato/alpm/remote"
)

func TestGetRepoFromDBFile(t *testing.T) {
	_, err := remote.GetRepoFromDBFile("core", "/var/lib/pacman/sync/core.db")
	if err != nil {
		t.Fatalf("Failed to get repo from db file: %v", err)
	}
}
