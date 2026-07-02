package stream

import (
	"errors"
	"io"
)

var errNilStream = errors.New("stream: nil underlying stream")

type FileStream struct {
	fileName    string
	contentType string
	stream      io.ReadSeekCloser
}

func NewFileStream(fileName string, contentType string, stream io.ReadSeekCloser) *FileStream {
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
		return 0, errNilStream
	}
	return f.stream.Seek(offset, whence)
}
