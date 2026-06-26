package aurweb

import (
	"io"
	"iter"
)

// maxResponseBytes caps how much of an upstream response we buffer, bounding
// memory against a hostile or broken upstream.
const maxResponseBytes = 32 << 20 // 32 MiB

func readAllLimited(r io.Reader) ([]byte, error) {
	return io.ReadAll(io.LimitReader(r, maxResponseBytes))
}

// slicesChunk iterates s in contiguous slices of at most n elements.
func slicesChunk[T any](s []T, n int) iter.Seq[[]T] {
	return func(yield func([]T) bool) {
		if n <= 0 {
			n = len(s)
		}
		for i := 0; i < len(s); i += n {
			if !yield(s[i:min(i+n, len(s))]) {
				return
			}
		}
	}
}
