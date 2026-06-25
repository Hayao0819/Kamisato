package joblog

import (
	"strings"
	"sync"
	"testing"
)

func TestBufferWriteReturnsLen(t *testing.T) {
	b := New(0)
	p := []byte("hello world")
	n, err := b.Write(p)
	if err != nil {
		t.Fatalf("Write err: %v", err)
	}
	if n != len(p) {
		t.Errorf("Write n = %d, want %d", n, len(p))
	}
}

func TestBufferCapEnforced(t *testing.T) {
	const max = 64
	b := New(max)

	// Write well past the cap.
	chunk := []byte(strings.Repeat("x", 50))
	totalWritten := 0
	for i := 0; i < 10; i++ {
		n, err := b.Write(chunk)
		if err != nil {
			t.Fatalf("Write err: %v", err)
		}
		if n != len(chunk) {
			t.Errorf("Write n = %d, want %d (must report full len even when dropped)", n, len(chunk))
		}
		totalWritten += n
	}

	got := b.String()
	// Len must never exceed max plus the one truncation marker.
	marker := "--- log truncated"
	if !strings.Contains(got, marker) {
		t.Fatalf("expected truncation marker, got %q", got)
	}
	if c := strings.Count(got, marker); c != 1 {
		t.Errorf("truncation marker appeared %d times, want exactly 1", c)
	}
	// Real data plus marker; must be far below totalWritten (the cap drops the
	// overwhelming majority of the 500 written bytes).
	if b.Len() >= totalWritten {
		t.Errorf("buffer len %d should be far below total written %d", b.Len(), totalWritten)
	}
	// Bounded: at most maxBytes overshot by one full write, plus the marker. Never
	// the full totalWritten.
	markerFull := "\n--- log truncated (max 64 bytes) ---\n"
	if upper := max + len(chunk) + len(markerFull); b.Len() > upper {
		t.Errorf("buffer len %d exceeds bound %d (cap + one write + marker)", b.Len(), upper)
	}
}

func TestBufferBytesFromDelta(t *testing.T) {
	b := New(0)
	b.Write([]byte("aaa"))

	chunk, total, closed := b.BytesFrom(0)
	if string(chunk) != "aaa" || total != 3 || closed {
		t.Fatalf("BytesFrom(0) = (%q,%d,%v), want (aaa,3,false)", chunk, total, closed)
	}

	b.Write([]byte("bbbb"))
	chunk, total, closed = b.BytesFrom(total)
	if string(chunk) != "bbbb" || total != 7 || closed {
		t.Fatalf("BytesFrom(3) = (%q,%d,%v), want (bbbb,7,false)", chunk, total, closed)
	}

	// No new bytes since offset == total.
	chunk, total, _ = b.BytesFrom(total)
	if len(chunk) != 0 || total != 7 {
		t.Errorf("BytesFrom at tail = (%q,%d), want empty chunk and total 7", chunk, total)
	}
}

func TestBufferBytesFromOffsetClamping(t *testing.T) {
	b := New(0)
	b.Write([]byte("data"))

	// Negative offset clamps to 0.
	chunk, total, _ := b.BytesFrom(-5)
	if string(chunk) != "data" || total != 4 {
		t.Errorf("BytesFrom(-5) = (%q,%d), want (data,4)", chunk, total)
	}

	// Offset beyond length clamps to total.
	chunk, total, _ = b.BytesFrom(99)
	if len(chunk) != 0 || total != 4 {
		t.Errorf("BytesFrom(99) = (%q,%d), want (empty,4)", chunk, total)
	}
}

func TestBufferBytesFromClosed(t *testing.T) {
	b := New(0)
	b.Write([]byte("x"))
	b.Close()
	_, _, closed := b.BytesFrom(0)
	if !closed {
		t.Error("BytesFrom should report closed after Close")
	}
}

// TestBufferConcurrentReadWrite exercises the lock under -race.
func TestBufferConcurrentReadWrite(t *testing.T) {
	b := New(1 << 20)
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 1000; i++ {
			b.Write([]byte("line\n"))
		}
		b.Close()
	}()

	for r := 0; r < 4; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			offset := 0
			for {
				chunk, total, closed := b.BytesFrom(offset)
				offset += len(chunk)
				if closed && len(chunk) == 0 {
					_ = total
					return
				}
			}
		}()
	}
	wg.Wait()
}
