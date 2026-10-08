//go:build integration

package git

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestCloneIntegration is an explicit opt-in check against a public repository;
// ordinary unit tests never depend on outbound network availability.
func TestCloneIntegration(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	target := filepath.Join(t.TempDir(), "repo")
	if err := Clone(ctx, CloneOptions{
		URL:    "https://github.com/octocat/Hello-World.git",
		Dir:    target,
		Depth:  1,
		Strict: true,
	}); err != nil {
		t.Fatalf("strict https clone failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(target, ".git")); err != nil {
		t.Fatalf("clone produced no .git dir: %v", err)
	}
}
