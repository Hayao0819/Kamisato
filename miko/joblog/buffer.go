// Package joblog provides a thread-safe, append-only log buffer that supports
// blocking incremental reads, so an HTTP handler can stream a build's logs over
// SSE while the worker is still writing them.
package joblog

import (
	"bytes"
	"sync"
)

// Buffer is an append-only log buffer. It implements io.Writer (the build
// backend writes to it) and lets readers wait for new bytes from an offset
// until the buffer is closed.
type Buffer struct {
	mu     sync.Mutex
	cond   *sync.Cond
	buf    bytes.Buffer
	closed bool
}

// New returns an empty, open Buffer.
func New() *Buffer {
	b := &Buffer{}
	b.cond = sync.NewCond(&b.mu)
	return b
}

// Write appends p and wakes any blocked readers. It never returns an error.
func (b *Buffer) Write(p []byte) (int, error) {
	b.mu.Lock()
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

// String returns the full contents accumulated so far.
func (b *Buffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// Len returns the number of bytes accumulated so far.
func (b *Buffer) Len() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Len()
}

// Closed reports whether Close has been called.
func (b *Buffer) Closed() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.closed
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
