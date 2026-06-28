// Package joblog provides a thread-safe, append-only log buffer that supports
// blocking incremental reads, so an HTTP handler can stream a build's logs over
// SSE while the worker is still writing them.
package joblog

import (
	"bytes"
	"fmt"
	"sync"
)

// Buffer is an append-only log buffer. It implements io.Writer (the build
// backend writes to it) and lets readers wait for new bytes from an offset
// until the buffer is closed.
type Buffer struct {
	mu sync.Mutex
	// maxBytes caps the accumulated log size to bound memory; <= 0 means
	// unbounded. Once reached, further writes are dropped after a one-time
	// truncation marker so a runaway build cannot exhaust memory.
	maxBytes  int
	truncated bool
	cond      *sync.Cond
	buf       bytes.Buffer
	closed    bool
}

// New returns an empty, open Buffer capped at maxBytes (<= 0 means unbounded).
func New(maxBytes int) *Buffer {
	b := &Buffer{maxBytes: maxBytes}
	b.cond = sync.NewCond(&b.mu)
	return b
}

// Write appends p and wakes any blocked readers. It never returns an error.
// Once the buffer reaches maxBytes the payload is dropped (a one-time truncation
// marker is appended instead) but Write still reports len(p) written so the
// caller's io.Copy does not abort.
func (b *Buffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	if b.maxBytes > 0 && b.buf.Len() >= b.maxBytes {
		if !b.truncated {
			b.truncated = true
			b.buf.WriteString(fmt.Sprintf("\n--- log truncated (max %d bytes) ---\n", b.maxBytes))
			b.mu.Unlock()
			b.cond.Broadcast()
			return len(p), nil
		}
		b.mu.Unlock()
		return len(p), nil
	}
	n, _ := b.buf.Write(p)
	b.mu.Unlock()
	b.cond.Broadcast()
	return n, nil
}

// Close marks the buffer complete and wakes all blocked readers. Subsequent
// writes still append but readers may stop once they observe closed.
func (b *Buffer) Close() {
	b.mu.Lock()
	b.closed = true
	b.mu.Unlock()
	b.cond.Broadcast()
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

func (b *Buffer) Closed() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.closed
}

// BytesFrom returns a copy of the bytes from offset onward without blocking,
// along with the current total length and the closed flag. offset is clamped to
// [0, total], mirroring WaitFrom. Callers advance their offset by the returned
// total, so each byte is emitted exactly once.
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

// WaitFrom blocks until there are bytes beyond offset or the buffer is closed,
// then returns a copy of the bytes from offset onward together with the closed
// flag. When closed and no new bytes remain it returns (nil, true). The caller
// advances offset by len(data) and loops until closed.
func (b *Buffer) WaitFrom(offset int) (data []byte, closed bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for b.buf.Len() <= offset && !b.closed {
		b.cond.Wait()
	}
	full := b.buf.Bytes()
	if offset < 0 {
		offset = 0
	}
	if offset > len(full) {
		offset = len(full)
	}
	out := make([]byte, len(full)-offset)
	copy(out, full[offset:])
	return out, b.closed
}
