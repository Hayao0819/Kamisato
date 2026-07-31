package localfs

import (
	"os"
	"path/filepath"
	"testing"
)

func TestObjectMover(t *testing.T) {
	root := t.TempDir()
	l := New(root, nil) // raw-key ops bypass the repo allowlist

	srcKey := "_pool_/objects/deadbeef"
	want := []byte("package bytes")
	if err := os.MkdirAll(filepath.Join(root, "_pool_", "objects"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "_pool_", "objects", "deadbeef"), want, 0o644); err != nil {
		t.Fatal(err)
	}

	dstKey := "myrepo/x86_64/foo-1.0-1-x86_64.pkg.tar.zst"
	if err := l.CopyObject(srcKey, dstKey); err != nil {
		t.Fatalf("CopyObject: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(root, "myrepo", "x86_64", "foo-1.0-1-x86_64.pkg.tar.zst"))
	if err != nil {
		t.Fatalf("read copied object: %v", err)
	}
	if string(got) != string(want) {
		t.Fatalf("copied bytes = %q, want %q", got, want)
	}

	keys, err := l.ListObjects("_pool_/objects/")
	if err != nil {
		t.Fatalf("ListObjects: %v", err)
	}
	if len(keys) != 1 || keys[0] != srcKey {
		t.Fatalf("ListObjects = %v, want [%s]", keys, srcKey)
	}

	if err := l.DeleteObject(srcKey); err != nil {
		t.Fatalf("DeleteObject: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "_pool_", "objects", "deadbeef")); !os.IsNotExist(err) {
		t.Fatalf("object still present after delete: %v", err)
	}
	// Deleting a missing key is a no-op.
	if err := l.DeleteObject(srcKey); err != nil {
		t.Fatalf("DeleteObject (missing) = %v, want nil", err)
	}

	// A traversal attempt is refused.
	if err := l.DeleteObject("../escape"); err == nil {
		t.Fatal("DeleteObject(../escape) = nil, want rejection")
	}
}

func TestCopyObjectFailurePreservesDestination(t *testing.T) {
	root := t.TempDir()
	l := New(root, nil)
	srcKey := "_pool_/objects/source"
	dstKey := "myrepo/x86_64/package.pkg.tar.zst"
	if err := os.MkdirAll(filepath.Join(root, filepath.FromSlash(srcKey)), 0o755); err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(root, filepath.FromSlash(dstKey))
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, []byte("published"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Opening a directory succeeds on Unix, but copying its bytes fails. A
	// direct O_TRUNC destination would destroy the published object first.
	if err := l.CopyObject(srcKey, dstKey); err == nil {
		t.Fatal("CopyObject(directory source) = nil, want copy error")
	}
	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "published" {
		t.Fatalf("failed copy changed destination to %q", got)
	}
}
