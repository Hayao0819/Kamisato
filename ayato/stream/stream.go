package stream

import (
	"io"
)

// FileStream is a file stream that implements ReadSeekCloser.
type FileStream struct {
	fileName    string
	contentType string
	stream      io.ReadSeekCloser
}

// NewFileStream creates a new FileStream.
func NewFileStream(fileName string, contentType string, stream io.ReadSeekCloser) *FileStream {
	return &FileStream{
		fileName:    fileName,
		contentType: contentType,
		stream:      stream,
	}
}

// Read reads data from the file.
func (f *FileStream) Read(p []byte) (n int, err error) {
	if f.stream != nil {
		return f.stream.Read(p)
	}
	return 0, nil
}

// Close closes the file.
func (f *FileStream) Close() error {
	if f.stream != nil {
		return f.stream.Close()
	}
	return nil
}

// FileName returns the file name.
func (f *FileStream) FileName() string {
	return f.fileName
}

// ContentType returns the MIME type.
func (f *FileStream) ContentType() string {
	if f.contentType != "" {
		return f.contentType
	}
	return "application/octet-stream"
}

// Seek seeks to a position in the file.
func (f *FileStream) Seek(offset int64, whence int) (int64, error) {
	return f.stream.Seek(offset, whence)
}
