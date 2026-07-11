package migrate_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/Hayao0819/Kamisato/ayato/migrate"
	"github.com/Hayao0819/Kamisato/ayato/repository/blob/localfs"
)

func TestUnpool(t *testing.T) {
	root := t.TempDir()
	kvStore := newStore(t)
	stores := &migrate.Stores{KV: kvStore, Blob: localfs.New(root, []string{"myrepo"})}

	// Seed a pooled object plus its pointer and the indices Contract must drop.
	if err := os.MkdirAll(filepath.Join(root, "_pool_", "objects"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "_pool_", "objects", "abc"), []byte("bytes"), 0o644); err != nil {
		t.Fatal(err)
	}
	dstKey := "myrepo/x86_64/foo-1.0-1-x86_64.pkg.tar.zst"
	_ = kvStore.Set("poolptr", dstKey, []byte("abc"), 0)
	_ = kvStore.Set("poolobj", "abc", []byte("{}"), 0)
	_ = kvStore.Set("pkgfile", "x86_64/foo", []byte(dstKey), 0)

	m := registered(t, 1)
	ctx := context.Background()

	if err := m.Expand(ctx, stores); err != nil {
		t.Fatalf("Expand: %v", err)
	}
	if b, err := os.ReadFile(filepath.Join(root, "myrepo", "x86_64", "foo-1.0-1-x86_64.pkg.tar.zst")); err != nil || string(b) != "bytes" {
		t.Fatalf("copied object = (%q, %v), want bytes", b, err)
	}

	if err := m.Contract(ctx, stores); err != nil {
		t.Fatalf("Contract: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "_pool_", "objects", "abc")); !os.IsNotExist(err) {
		t.Fatalf("pool object still present: %v", err)
	}
	for _, ns := range []string{"poolptr", "poolobj", "pkgfile"} {
		if got, _ := kvStore.List(ns); len(got) != 0 {
			t.Fatalf("%s not cleared: %v", ns, got)
		}
	}
}

func registered(t *testing.T, version int) migrate.Migration {
	t.Helper()
	for _, m := range migrate.Registered() {
		if m.Version() == version {
			return m
		}
	}
	t.Fatalf("migration %d not registered", version)
	return nil
}
