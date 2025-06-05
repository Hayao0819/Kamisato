package remoterepo_test

import (
	"testing"

	remote "github.com/Hayao0819/Kamisato/pkg/alpm/remoterepo"
)

func TestGetRepoFromDBFile(t *testing.T) {
	_, err := remote.GetRepoFromDBFile("core", "/var/lib/pacman/sync/core.db")
	if err != nil {
		t.Fatalf("Failed to get repo from db file: %v", err)
	}
}
