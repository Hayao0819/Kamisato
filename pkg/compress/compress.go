// Package compress sniffs and transparently decodes the compression formats
// pacman uses for its repo databases and package archives.
package compress

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"io"

	"github.com/klauspost/compress/zstd"
	"github.com/ulikunitz/xz"
)

// DetectCompression sniffs the gzip/zstd/xz magic bytes of r and returns a decoding
// ReadCloser plus the format name, or r unchanged with an empty name when uncompressed.
func DetectCompression(r io.Reader) (io.ReadCloser, string, error) {
	br := bufio.NewReader(r)
	peek, err := br.Peek(6) // magic bytes for gzip, zstd, xz are at most 6 bytes
	if err != nil && err != io.EOF {
		return nil, "", err
	}

	if len(peek) >= 2 && peek[0] == 0x1f && peek[1] == 0x8b {
		gzipReader, err := gzip.NewReader(br)
		if err != nil {
			return nil, "gzip", err
		}
		return gzipReader, "gzip", nil
	}

	if len(peek) >= 4 && bytes.Equal(peek[:4], []byte{0x28, 0xb5, 0x2f, 0xfd}) {
		zstdReader, err := zstd.NewReader(br)
		if err != nil {
			return nil, "zstd", err
		}
		// IOReadCloser's Close stops the decoder's background goroutine; NopCloser would let it race the source.
		return zstdReader.IOReadCloser(), "zstd", nil
	}

	if len(peek) >= 6 && bytes.Equal(peek[:6], []byte{0xfd, 0x37, 0x7a, 0x58, 0x5a, 0x00}) {
		xzReader, err := xz.NewReader(br)
		if err != nil {
			return nil, "xz", err
		}
		return io.NopCloser(xzReader), "xz", nil
	}

	return io.NopCloser(br), "", nil
}
