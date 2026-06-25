package utils

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"io"

	"github.com/klauspost/compress/zstd"
	"github.com/ulikunitz/xz"
)

// DetectCompression detects the compression format (gzip, zstd, xz) of the given io.Reader
// and returns an io.ReadCloser wrapped in a decoder for the detected format.
// If no compression format is detected, it returns the original io.Reader wrapped in an io.NopCloser.
func DetectCompression(r io.Reader) (io.ReadCloser, string, error) {
	// buffer so we can peek the first few bytes to determine the compression format
	br := bufio.NewReader(r)
	peek, err := br.Peek(6) // magic bytes for gzip, zstd, xz are at most 6 bytes
	if err != nil && err != io.EOF {
		return nil, "", err
	}

	// detect gzip format (magic bytes: 1f 8b)
	if len(peek) >= 2 && peek[0] == 0x1f && peek[1] == 0x8b {
		gzipReader, err := gzip.NewReader(br)
		if err != nil {
			return nil, "gzip", err
		}
		return gzipReader, "gzip", nil
	}

	// detect zstd format (magic bytes: 28 b5 2f fd)
	if len(peek) >= 4 && bytes.Equal(peek[:4], []byte{0x28, 0xb5, 0x2f, 0xfd}) {
		zstdReader, err := zstd.NewReader(br)
		if err != nil {
			return nil, "zstd", err
		}
		// IOReadCloser's Close stops the decoder's background goroutine; a plain
		// NopCloser would leave it reading the source after the caller closes,
		// racing anyone who reuses (e.g. seeks) the underlying stream.
		return zstdReader.IOReadCloser(), "zstd", nil
	}

	// detect xz format (magic bytes: fd 37 7a 58 5a 00)
	if len(peek) >= 6 && bytes.Equal(peek[:6], []byte{0xfd, 0x37, 0x7a, 0x58, 0x5a, 0x00}) {
		xzReader, err := xz.NewReader(br)
		if err != nil {
			return nil, "xz", err
		}
		return io.NopCloser(xzReader), "xz", nil
	}

	// no compression format detected
	return io.NopCloser(br), "", nil
}
