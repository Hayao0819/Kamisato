package stream

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestRewind(t *testing.T) {
	reader := bytes.NewReader([]byte("content"))
	if _, err := reader.Seek(4, io.SeekStart); err != nil {
		t.Fatal(err)
	}
	if err := Rewind(reader); err != nil {
		t.Fatalf("Rewind: %v", err)
	}
	content, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "content" {
		t.Fatalf("content after Rewind = %q", content)
	}
}

func TestOpenFileWithType(t *testing.T) {
	cases := []struct {
		name   string
		magic  []byte
		wantCT string
	}{
		{"gzip", []byte{0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x03}, "application/gzip"},
		{"zstd", []byte{0x28, 0xb5, 0x2f, 0xfd, 0x00, 0x00, 0x00, 0x00}, "application/zstd"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := filepath.Join(t.TempDir(), tc.name)
			if err := os.WriteFile(p, tc.magic, 0o600); err != nil {
				t.Fatalf("WriteFile: %v", err)
			}
			fs, err := OpenFileWithType(p)
			if err != nil {
				t.Fatalf("OpenFileWithType: %v", err)
			}
			defer fs.Close()
			if fs.FileName() != p {
				t.Fatalf("FileName = %q, want %q", fs.FileName(), p)
			}
			if fs.ContentType() != tc.wantCT {
				t.Fatalf("ContentType = %q, want %q", fs.ContentType(), tc.wantCT)
			}
			got, err := io.ReadAll(fs)
			if err != nil {
				t.Fatalf("ReadAll: %v", err)
			}
			if !bytes.Equal(got, tc.magic) {
				t.Fatalf("Read = %x, want %x", got, tc.magic)
			}
		})
	}
}

func TestOpenFileWithTypeMissing(t *testing.T) {
	if _, err := OpenFileWithType(filepath.Join(t.TempDir(), "nope")); err == nil {
		t.Fatal("OpenFileWithType on missing path = nil error, want error")
	}
}
