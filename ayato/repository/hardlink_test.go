package repository

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Hayao0819/Kamisato/ayato/stream"
)

// onDiskSeek adds an on-disk path to a SeekFile so writeSeekFileToPath can hardlink it.
type onDiskSeek struct {
	stream.SeekFile
	path string
}

func (o onDiskSeek) OnDiskPath() string { return o.path }

func TestWriteSeekFileToPath_HardlinksOnDiskSource(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.pkg.tar.zst")
	content := []byte("package bytes")
	if err := os.WriteFile(src, content, 0o644); err != nil {
		t.Fatal(err)
	}
	f := onDiskSeek{SeekFile: openSeek(t, src), path: src}

	dst := filepath.Join(dir, "dst.pkg.tar.zst")
	if err := writeSeekFileToPath(dst, f); err != nil {
		t.Fatalf("writeSeekFileToPath: %v", err)
	}

	si, err := os.Stat(src)
	if err != nil {
		t.Fatal(err)
	}
	di, err := os.Stat(dst)
	if err != nil {
		t.Fatal(err)
	}
	if !os.SameFile(si, di) {
		t.Fatal("dst is not a hardlink of src (want same inode, no copy)")
	}
	if got, _ := os.ReadFile(dst); string(got) != string(content) {
		t.Fatalf("dst content = %q, want %q", got, content)
	}
}

func TestWriteSeekFileToPath_CopiesPlainStream(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.pkg.tar.zst")
	content := []byte("stream bytes")
	if err := os.WriteFile(src, content, 0o644); err != nil {
		t.Fatal(err)
	}
	f := openSeek(t, src) // plain SeekFile: no OnDiskPath, so it must be copied

	dst := filepath.Join(dir, "dst.pkg.tar.zst")
	if err := writeSeekFileToPath(dst, f); err != nil {
		t.Fatalf("writeSeekFileToPath: %v", err)
	}

	si, _ := os.Stat(src)
	di, _ := os.Stat(dst)
	if os.SameFile(si, di) {
		t.Fatal("a plain stream should be copied, not hardlinked")
	}
	if got, _ := os.ReadFile(dst); string(got) != string(content) {
		t.Fatalf("dst content = %q, want %q", got, content)
	}
}
