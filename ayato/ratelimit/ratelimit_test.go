package ratelimit

import (
	"testing"
	"time"

	"github.com/Hayao0819/Kamisato/ayato/repository/kv/badgerkv"
)

func newBadger(t *testing.T) *badgerkv.Store {
	t.Helper()
	s, err := badgerkv.New(t.TempDir())
	if err != nil {
		t.Fatalf("open badger: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestAllowUnderLimitThenReject(t *testing.T) {
	l := New(newBadger(t))
	const limit = 3
	window := time.Hour

	for i := 1; i <= limit; i++ {
		if ok, _ := l.Allow("s", "10.0.0.1", limit, window); !ok {
			t.Fatalf("request %d: rejected, want allowed", i)
		}
	}
	ok, retry := l.Allow("s", "10.0.0.1", limit, window)
	if ok {
		t.Fatal("over-limit request: allowed, want rejected")
	}
	if retry <= 0 || retry > window {
		t.Fatalf("retry hint = %v, want within (0, %v]", retry, window)
	}
}

func TestScopeAndClientIsolated(t *testing.T) {
	l := New(newBadger(t))
	if ok, _ := l.Allow("a", "1.1.1.1", 1, time.Hour); !ok {
		t.Fatal("scope a, ip1: first rejected")
	}
	if ok, _ := l.Allow("a", "1.1.1.1", 1, time.Hour); ok {
		t.Fatal("scope a, ip1: second allowed, want rejected")
	}
	// A different scope and a different client each keep an independent budget.
	if ok, _ := l.Allow("b", "1.1.1.1", 1, time.Hour); !ok {
		t.Fatal("scope b, ip1: first rejected (independent counter)")
	}
	if ok, _ := l.Allow("a", "2.2.2.2", 1, time.Hour); !ok {
		t.Fatal("scope a, ip2: first rejected (independent counter)")
	}
}

func TestWindowResets(t *testing.T) {
	clock := time.Unix(1_700_000_000, 0)
	l := New(newBadger(t))
	l.now = func() time.Time { return clock }

	window := time.Minute
	if ok, _ := l.Allow("s", "ip", 1, window); !ok {
		t.Fatal("first request rejected")
	}
	if ok, _ := l.Allow("s", "ip", 1, window); ok {
		t.Fatal("second request in same window allowed, want rejected")
	}
	// Advance past the window: a fresh counter key starts the budget over.
	clock = clock.Add(window + time.Second)
	if ok, _ := l.Allow("s", "ip", 1, window); !ok {
		t.Fatal("request after window reset rejected, want allowed")
	}
}

func TestSharedAcrossInstances(t *testing.T) {
	// Two limiters over ONE kv model two Cloud Run replicas: the limit must hold
	// across both, not be granted twice.
	store := newBadger(t)
	a := New(store)
	b := New(store)

	const limit = 4
	window := time.Hour
	allowed := 0
	for i := 0; i < 10; i++ {
		l := a
		if i%2 == 1 {
			l = b
		}
		if ok, _ := l.Allow("s", "ip", limit, window); ok {
			allowed++
		}
	}
	if allowed != limit {
		t.Fatalf("shared limit: %d requests allowed across both instances, want %d", allowed, limit)
	}
}

func TestDisabledWhenNonPositive(t *testing.T) {
	l := New(newBadger(t))
	if ok, _ := l.Allow("s", "ip", 0, time.Hour); !ok {
		t.Fatal("limit 0 should disable limiting")
	}
	if ok, _ := l.Allow("s", "ip", 5, 0); !ok {
		t.Fatal("window 0 should disable limiting")
	}
}
