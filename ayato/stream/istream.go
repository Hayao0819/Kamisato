package stream

import "io"

// commonFileStream is an interface for obtaining file name and MIME type.
type commonFileStream interface {
	FileName() string
	ContentType() string
}

// IFileStream is a streaming file interface (Read/Close + meta information).
type IFileStream interface {
	io.ReadCloser
	commonFileStream
}

// IFileSeekStream is a seekable streaming file interface.
type IFileSeekStream interface {
	io.ReadSeekCloser
	commonFileStream
}
