package raiou

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadInlineSkipsAndReports(t *testing.T) {
	dir := t.TempDir()
	write := func(name string, size int) {
		if err := os.WriteFile(filepath.Join(dir, name), make([]byte, size), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	write("PKGBUILD", 10)
	write(".SRCINFO", 10)
	write("build.log", 10)
	write("foo.pkg.tar.zst", 10)
	write("blob.tar", MaxInlineSource+1)
	write("sidecar.patch", 10)

	var skipped []string
	pkgbuild, files, err := ReadInline(dir, func(name string, _ int64) {
		skipped = append(skipped, name)
	})
	if err != nil {
		t.Fatalf("ReadInline: %v", err)
	}
	if pkgbuild == "" {
		t.Error("PKGBUILD not returned")
	}
	if len(files) != 1 {
		t.Errorf("files = %v, want only sidecar.patch", files)
	}
	if _, ok := files["sidecar.patch"]; !ok {
		t.Errorf("sidecar.patch missing from %v", files)
	}
	if len(skipped) != 1 || skipped[0] != "blob.tar" {
		t.Errorf("onSkipLarge fired for %v, want [blob.tar]", skipped)
	}
}

func TestReadInlineErrorsWithoutPKGBUILD(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "sidecar.patch"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, _, err := ReadInline(dir, nil); err == nil {
		t.Error("ReadInline without PKGBUILD = nil, want error")
	}
}
