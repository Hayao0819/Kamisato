package artifact

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestIsPackageFile(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		want     bool
	}{
		{"zst package", "linux-6.1.0-1-x86_64.pkg.tar.zst", true},
		{"xz package", "linux-6.1.0-1-x86_64.pkg.tar.xz", true},
		{"gz package", "linux-6.1.0-1-x86_64.pkg.tar.gz", true},
		{"bz2 package", "linux-6.1.0-1-x86_64.pkg.tar.bz2", true},
		{"uncompressed package", "linux-6.1.0-1-x86_64.pkg.tar", true},
		{"sig file", "linux-6.1.0-1-x86_64.pkg.tar.zst.sig", false},
		{"plain tarball", "archive.tar.gz", false},
		{"random file", "README.md", false},
		{"empty string", "", false},
		{"extension only", ".pkg.tar.zst", false},
		{"PKGBUILD", "PKGBUILD", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsPackageFile(tt.filename); got != tt.want {
				t.Errorf("IsPackageFile(%q) = %v, want %v", tt.filename, got, tt.want)
			}
		})
	}
}

func TestCollectDetectsSameNameOverwrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "pkg-1-1-x86_64.pkg.tar.zst")
	if err := os.WriteFile(path, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	oldTime := time.Now().Add(-time.Hour)
	if err := os.Chtimes(path, oldTime, oldTime); err != nil {
		t.Fatal(err)
	}
	baseline, err := Snapshot(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := Collect(dir, baseline); err != nil || len(got) != 0 {
		t.Fatalf("unchanged package collected: got=%v err=%v", got, err)
	}

	if err := os.WriteFile(path, []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, time.Now(), time.Now()); err != nil {
		t.Fatal(err)
	}
	got, err := Collect(dir, baseline)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != path {
		t.Fatalf("same-name overwrite not collected: %v", got)
	}
}
