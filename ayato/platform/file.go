package platform

import (
	"io"
	"os"

	"github.com/Hayao0819/Kamisato/internal/errors"
	"github.com/gabriel-vasile/mimetype"
)

var errNilFileStream = errors.New("platform: nil underlying file stream")

// File is a named, typed stream whose ownership can be released by its caller.
type File interface {
	io.ReadCloser
	FileName() string
	ContentType() string
}

// SeekFile is a File that consumers can rewind for validation and persistence.
type SeekFile interface {
	File
	io.Seeker
}

// OnDiskFile is an optional SeekFile capability: the bytes already live in a real
// file at OnDiskPath, so a consumer that needs them on disk can hardlink instead
// of copying.
type OnDiskFile interface {
	OnDiskPath() string
}

// FileStream attaches transport metadata to an owned seekable stream.
type FileStream struct {
	fileName    string
	contentType string
	stream      io.ReadSeekCloser
}

func NewFileStream(
	fileName string,
	contentType string,
	stream io.ReadSeekCloser,
) *FileStream {
	return &FileStream{
		fileName:    fileName,
		contentType: contentType,
		stream:      stream,
	}
}

func (f *FileStream) Read(p []byte) (n int, err error) {
	if f.stream == nil {
		return 0, io.EOF
	}
	return f.stream.Read(p)
}

func (f *FileStream) Close() error {
	if f.stream != nil {
		return f.stream.Close()
	}
	return nil
}

func (f *FileStream) FileName() string {
	return f.fileName
}

func (f *FileStream) ContentType() string {
	if f.contentType != "" {
		return f.contentType
	}
	return "application/octet-stream"
}

func (f *FileStream) Seek(offset int64, whence int) (int64, error) {
	if f.stream == nil {
		return 0, errNilFileStream
	}
	return f.stream.Seek(offset, whence)
}

func Rewind(seeker io.Seeker) error {
	_, err := seeker.Seek(0, io.SeekStart)
	return err
}

func OpenFileWithType(filePath string) (*FileStream, error) {
	mt, err := mimetype.DetectFile(filePath)
	if err != nil {
		return nil, err
	}
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	return NewFileStream(file.Name(), mt.String(), file), nil
}
