package safefile

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLockContextHonorsContentionAndCancellation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.lock")
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatal(err)
	}

	first, err := Lock(path, 0o600)
	if err != nil {
		t.Fatalf("Lock first: %v", err)
	}
	defer func() { _ = first.Unlock() }()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	if _, err := LockContext(ctx, path, 0o600, time.Millisecond); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("LockContext error = %v, want deadline exceeded", err)
	}

	if err := first.Unlock(); err != nil {
		t.Fatalf("Unlock first: %v", err)
	}
	second, err := LockContext(context.Background(), path, 0o600, time.Millisecond)
	if err != nil {
		t.Fatalf("LockContext after unlock: %v", err)
	}
	if err := second.Unlock(); err != nil {
		t.Fatalf("Unlock second: %v", err)
	}
	if err := second.Unlock(); err != nil {
		t.Fatalf("second Unlock call: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("lock mode = %#o, want 0600", got)
	}
}
