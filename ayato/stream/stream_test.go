package stream

import (
	"bytes"
	"io"
	"testing"

	"github.com/Hayao0819/Kamisato/internal/errors"
)

var (
	_ SeekFile = (*FileStream)(nil)
	_ File     = (*FileStream)(nil)
)

type fakeStream struct {
	*bytes.Reader
	closeErr error
	closed   bool
}

func newFakeStream(b []byte) *fakeStream {
	return &fakeStream{Reader: bytes.NewReader(b)}
}

func (f *fakeStream) Close() error {
	f.closed = true
	return f.closeErr
}

func TestReadRoundTrip(t *testing.T) {
	content := []byte("hello stream world")
	fs := NewFileStream("pkg.tar.zst", "application/zstd", newFakeStream(content))
	got, err := io.ReadAll(fs)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if !bytes.Equal(got, content) {
		t.Fatalf("Read = %q, want %q", got, content)
	}
}

func TestContentType(t *testing.T) {
	if ct := NewFileStream("a", "application/zstd", newFakeStream(nil)).ContentType(); ct != "application/zstd" {
		t.Fatalf("ContentType = %q, want application/zstd", ct)
	}
	if ct := NewFileStream("a", "", newFakeStream(nil)).ContentType(); ct != "application/octet-stream" {
		t.Fatalf("ContentType fallback = %q, want application/octet-stream", ct)
	}
}

func TestFileName(t *testing.T) {
	if name := NewFileStream("myrepo.db", "", newFakeStream(nil)).FileName(); name != "myrepo.db" {
		t.Fatalf("FileName = %q, want myrepo.db", name)
	}
}

func TestClose(t *testing.T) {
	want := errors.New("close boom")
	fs := NewFileStream("a", "", &fakeStream{Reader: bytes.NewReader(nil), closeErr: want})
	if err := fs.Close(); !errors.Is(err, want) {
		t.Fatalf("Close = %v, want %v", err, want)
	}
	if err := NewFileStream("a", "", nil).Close(); err != nil {
		t.Fatalf("Close on nil stream = %v, want nil", err)
	}
}

func TestSeekDelegates(t *testing.T) {
	content := []byte("0123456789")
	fs := NewFileStream("a", "", newFakeStream(content))
	if _, err := fs.Seek(4, io.SeekStart); err != nil {
		t.Fatalf("Seek: %v", err)
	}
	rest, err := io.ReadAll(fs)
	if err != nil {
		t.Fatalf("ReadAll after seek: %v", err)
	}
	if string(rest) != "456789" {
		t.Fatalf("remainder = %q, want 456789", rest)
	}
}

func TestSeekNilStream(t *testing.T) {
	if _, err := NewFileStream("a", "", nil).Seek(0, io.SeekStart); err == nil {
		t.Fatal("Seek on nil stream = nil error, want error")
	}
}

func TestReadNilStreamEOF(t *testing.T) {
	n, err := NewFileStream("a", "", nil).Read(make([]byte, 8))
	if n != 0 || !errors.Is(err, io.EOF) {
		t.Fatalf("Read on nil stream = (%d, %v), want (0, EOF)", n, err)
	}
}

func TestCopyDrain(t *testing.T) {
	content := []byte("download drain payload")
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, NewFileStream("a", "", newFakeStream(content))); err != nil {
		t.Fatalf("Copy: %v", err)
	}
	if !bytes.Equal(buf.Bytes(), content) {
		t.Fatalf("drained = %q, want %q", buf.Bytes(), content)
	}

	n, err := io.Copy(io.Discard, NewFileStream("a", "", nil))
	if n != 0 || err != nil {
		t.Fatalf("Copy on nil stream = (%d, %v), want (0, nil)", n, err)
	}
}
