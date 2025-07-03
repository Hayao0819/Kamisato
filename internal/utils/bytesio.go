package utils

import (
	"bytes"
	"io"
)

type ReadSeekCloser struct {
	io.ReadSeeker
}

func (r ReadSeekCloser) Close() error {
	return nil
}

func BufferToReadSeekCloser(buf *bytes.Buffer) io.ReadSeekCloser {
	return ReadSeekCloser{
		ReadSeeker: bytes.NewReader(buf.Bytes()),
	}
}
