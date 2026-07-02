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
