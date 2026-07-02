package utils

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCopyDir(t *testing.T) {
	src := t.TempDir()
	if err := os.MkdirAll(filepath.Join(src, "a", "b"), 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"top.txt":                           "top",
		filepath.Join("a", "mid.txt"):       "mid",
		filepath.Join("a", "b", "leaf.txt"): "leaf",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(src, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// A restrictive mode proves copy.Copy preserves the source permission bits.
	if err := os.Chmod(filepath.Join(src, "a", "b", "leaf.txt"), 0o640); err != nil {
		t.Fatal(err)
	}

	dst := filepath.Join(t.TempDir(), "copy")
	if err := CopyDir(src, dst); err != nil {
		t.Fatalf("CopyDir: %v", err)
	}

	for name, content := range files {
		got, err := os.ReadFile(filepath.Join(dst, name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		if string(got) != content {
			t.Errorf("%s: got %q, want %q", name, got, content)
		}
	}

	info, err := os.Stat(filepath.Join(dst, "a", "b", "leaf.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o640 {
		t.Errorf("leaf mode: got %o, want 0640", got)
	}
}

func TestMoveFilePreservesMode(t *testing.T) {
	src := filepath.Join(t.TempDir(), "src.bin")
	if err := os.WriteFile(src, []byte("payload"), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(src, 0o640); err != nil {
		t.Fatal(err)
	}

	// Move into a distinct directory that does not yet exist.
	dst := filepath.Join(t.TempDir(), "nested", "dst.bin")
	if err := MoveFile(src, dst); err != nil {
		t.Fatalf("MoveFile: %v", err)
	}

	if _, err := os.Stat(src); !os.IsNotExist(err) {
		t.Errorf("source still present: err=%v", err)
	}
	info, err := os.Stat(dst)
	if err != nil {
		t.Fatalf("stat dst: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o640 {
		t.Errorf("dst mode: got %o, want 0640", got)
	}
}
