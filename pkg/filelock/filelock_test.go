package filelock

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestAcquireContextHonorsContentionAndCancellation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.lock")
	first, err := Acquire(path, 0o600)
	if err != nil {
		t.Fatalf("Acquire first: %v", err)
	}
	defer func() { _ = first.Release() }()

	if guard, acquired, err := TryAcquire(path, 0o600); err != nil {
		t.Fatalf("TryAcquire: %v", err)
	} else if acquired {
		_ = guard.Release()
		t.Fatal("TryAcquire acquired a contended lock")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	if _, err := AcquireContext(ctx, path, 0o600, time.Millisecond); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("AcquireContext error = %v, want deadline exceeded", err)
	}

	if err := first.Release(); err != nil {
		t.Fatalf("Release first: %v", err)
	}
	second, err := AcquireContext(context.Background(), path, 0o600, time.Millisecond)
	if err != nil {
		t.Fatalf("AcquireContext after release: %v", err)
	}
	if err := second.Release(); err != nil {
		t.Fatalf("Release second: %v", err)
	}
	if err := second.Release(); err != nil {
		t.Fatalf("second Release call: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("lock mode = %#o, want 0600", got)
	}
}
