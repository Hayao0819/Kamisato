package pkg

import (
	"context"
	"errors"
	"io"

	"github.com/mholt/archives"
)

// DetectCompression sniffs r's compression format (the gzip/zstd/xz that pacman
// repo databases and package archives use) and returns a decoding ReadCloser
// plus the format's extension (e.g. ".gz", ".zst", ".xz"), or r unchanged with
// an empty name when it is not a recognized compressed stream. Detection and
// decoding are delegated to mholt/archives.
func DetectCompression(r io.Reader) (io.ReadCloser, string, error) {
	return DetectCompressionContext(context.Background(), r)
}

// DetectCompressionContext is DetectCompression with cancellation support for
// callers parsing data received from a remote source.
func DetectCompressionContext(ctx context.Context, r io.Reader) (io.ReadCloser, string, error) {
	format, stream, err := archives.Identify(ctx, "", r)
	if err != nil {
		if errors.Is(err, archives.NoMatch) {
			if stream == nil {
				stream = r
			}
			return io.NopCloser(stream), "", nil
		}
		return nil, "", err
	}
	decomp, ok := format.(archives.Decompressor)
	if !ok {
		if stream == nil {
			stream = r
		}
		return io.NopCloser(stream), "", nil
	}
	rc, err := decomp.OpenReader(stream)
	if err != nil {
		return nil, format.Extension(), err
	}
	return rc, format.Extension(), nil
}
