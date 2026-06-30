package shared

import "testing"

func TestRequirePinnedCommit(t *testing.T) {
	if err := (Resolved{Commit: "abc123"}).RequirePinnedCommit(); err != nil {
		t.Errorf("RequirePinnedCommit with a commit = %v, want nil", err)
	}
	if err := (Resolved{Commit: ""}).RequirePinnedCommit(); err == nil {
		t.Error("RequirePinnedCommit with no commit = nil, want error")
	}
}
