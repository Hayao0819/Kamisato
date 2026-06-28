package stream

import "io"

type File interface {
	io.ReadCloser
	FileName() string
	ContentType() string
}

type SeekFile interface {
	io.ReadSeekCloser
	FileName() string
	ContentType() string
}
