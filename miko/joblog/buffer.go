// Package joblog provides a thread-safe, append-only log buffer that supports
// incremental reads from an offset, so an HTTP handler can stream a build's logs
// over SSE while the worker is still writing them.
package joblog

import (
	"bytes"
	"fmt"
	"sync"
)

// Buffer is an append-only log buffer implementing io.Writer; readers poll for
// new bytes from an offset until it is closed.
type Buffer struct {
	mu sync.Mutex
	// maxBytes caps accumulated log size to bound memory (<= 0 means unbounded).
	// Past the cap, writes are dropped after a one-time truncation marker.
	maxBytes  int
	truncated bool
	buf       bytes.Buffer
	closed    bool
}

// New returns an empty, open Buffer capped at maxBytes (<= 0 means unbounded).
func New(maxBytes int) *Buffer {
	return &Buffer{maxBytes: maxBytes}
}

// Write appends p; it never errors. Past maxBytes the payload is dropped, but
// Write still reports len(p) so the caller's io.Copy does not abort.
func (b *Buffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	requested := len(p)
	if b.closed || b.truncated {
		return requested, nil
	}
	if b.maxBytes <= 0 {
		_, _ = b.buf.Write(p)
		return requested, nil
	}
	marker := []byte(fmt.Sprintf("\n--- log truncated (max %d bytes) ---\n", b.maxBytes))
	if len(marker) > b.maxBytes {
		marker = marker[:b.maxBytes]
	}
	payloadLimit := b.maxBytes - len(marker)
	remaining := payloadLimit - b.buf.Len()
	if len(p) <= remaining {
		_, _ = b.buf.Write(p)
		return requested, nil
	}
	if remaining > 0 {
		_, _ = b.buf.Write(p[:remaining])
	}
	_, _ = b.buf.Write(marker)
	b.truncated = true
	return requested, nil
}

// Close marks the buffer complete.
func (b *Buffer) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.closed = true
}

func (b *Buffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func (b *Buffer) Len() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Len()
}

// BytesFrom returns a copy of the bytes from offset onward without blocking, plus
// the total length and closed flag. offset is clamped to [0, total]; callers
// advance offset past the bytes they consume so each byte is emitted exactly once.
func (b *Buffer) BytesFrom(offset int) (data []byte, total int, closed bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	closed = b.closed
	full := b.buf.Bytes()
	total = len(full)
	if offset < 0 {
		offset = 0
	}
	if offset > total {
		offset = total
	}
	out := make([]byte, total-offset)
	copy(out, full[offset:])
	return out, total, closed
}
