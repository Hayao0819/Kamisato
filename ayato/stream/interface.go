package stream

import "io"

// File is a read-only file stream with metadata.
type File interface {
	io.ReadCloser
	FileName() string
	ContentType() string
}

// SeekFile is a seekable file stream with metadata.
type SeekFile interface {
	io.ReadSeekCloser
	FileName() string
	ContentType() string
}
