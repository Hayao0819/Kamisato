package platform

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/Hayao0819/Kamisato/internal/errors"
)

var (
	_ SeekFile = (*FileStream)(nil)
	_ File     = (*FileStream)(nil)
)

type fakeFileStream struct {
	*bytes.Reader
	closeErr error
	closed   bool
}

func newFakeFileStream(body []byte) *fakeFileStream {
	return &fakeFileStream{Reader: bytes.NewReader(body)}
}

func (f *fakeFileStream) Close() error {
	f.closed = true
	return f.closeErr
}

func TestFileStreamMetadataAndRead(t *testing.T) {
	content := []byte("hello stream world")
	file := NewFileStream(
		"pkg.tar.zst",
		"application/zstd",
		newFakeFileStream(content),
	)
	got, err := io.ReadAll(file)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if !bytes.Equal(got, content) {
		t.Fatalf("Read = %q, want %q", got, content)
	}
	if file.FileName() != "pkg.tar.zst" {
		t.Fatalf("FileName = %q, want pkg.tar.zst", file.FileName())
	}
	if file.ContentType() != "application/zstd" {
		t.Fatalf("ContentType = %q, want application/zstd", file.ContentType())
	}
	if got := NewFileStream("a", "", nil).ContentType(); got != "application/octet-stream" {
		t.Fatalf("ContentType fallback = %q, want application/octet-stream", got)
	}
}

func TestFileStreamClose(t *testing.T) {
	want := errors.New("close boom")
	file := NewFileStream(
		"a",
		"",
		&fakeFileStream{Reader: bytes.NewReader(nil), closeErr: want},
	)
	if err := file.Close(); !errors.Is(err, want) {
		t.Fatalf("Close = %v, want %v", err, want)
	}
	if err := NewFileStream("a", "", nil).Close(); err != nil {
		t.Fatalf("Close on nil stream = %v, want nil", err)
	}
}

func TestFileStreamSeek(t *testing.T) {
	content := []byte("0123456789")
	file := NewFileStream("a", "", newFakeFileStream(content))
	if _, err := file.Seek(4, io.SeekStart); err != nil {
		t.Fatalf("Seek: %v", err)
	}
	rest, err := io.ReadAll(file)
	if err != nil {
		t.Fatalf("ReadAll after seek: %v", err)
	}
	if string(rest) != "456789" {
		t.Fatalf("remainder = %q, want 456789", rest)
	}
	if _, err := NewFileStream("a", "", nil).Seek(0, io.SeekStart); err == nil {
		t.Fatal("Seek on nil stream = nil error, want error")
	}
}

func TestNilFileStreamReadsAsEmpty(t *testing.T) {
	file := NewFileStream("a", "", nil)
	n, err := file.Read(make([]byte, 8))
	if n != 0 || !errors.Is(err, io.EOF) {
		t.Fatalf("Read = (%d, %v), want (0, EOF)", n, err)
	}
	n64, err := io.Copy(io.Discard, file)
	if n64 != 0 || err != nil {
		t.Fatalf("Copy = (%d, %v), want (0, nil)", n64, err)
	}
}

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
			filePath := filepath.Join(t.TempDir(), tc.name)
			if err := os.WriteFile(filePath, tc.magic, 0o600); err != nil {
				t.Fatalf("WriteFile: %v", err)
			}
			file, err := OpenFileWithType(filePath)
			if err != nil {
				t.Fatalf("OpenFileWithType: %v", err)
			}
			defer func() { _ = file.Close() }()
			if file.FileName() != filePath {
				t.Fatalf("FileName = %q, want %q", file.FileName(), filePath)
			}
			if file.ContentType() != tc.wantCT {
				t.Fatalf("ContentType = %q, want %q", file.ContentType(), tc.wantCT)
			}
			got, err := io.ReadAll(file)
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
