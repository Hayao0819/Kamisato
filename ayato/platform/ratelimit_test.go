package platform

import (
	"testing"
	"time"

	"github.com/Hayao0819/Kamisato/ayato/repository/kv"
	"github.com/Hayao0819/Kamisato/ayato/repository/kv/badgerkv"
	sharedlimit "github.com/Hayao0819/Kamisato/pkg/ratelimit"
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
	l := NewRateLimiter(newBadger(t), kv.ErrNotFound)
	policy := sharedlimit.Policy{Limit: 3, Window: time.Hour}

	for i := 1; i <= policy.Limit; i++ {
		if decision := l.Allow("s", "10.0.0.1", policy); !decision.Allowed {
			t.Fatalf("request %d: rejected, want allowed", i)
		}
	}
	decision := l.Allow("s", "10.0.0.1", policy)
	if decision.Allowed {
		t.Fatal("over-limit request: allowed, want rejected")
	}
	if decision.RetryAfter <= 0 || decision.RetryAfter > policy.Window {
		t.Fatalf("retry hint = %v, want within (0, %v]", decision.RetryAfter, policy.Window)
	}
}

func TestScopeAndClientIsolated(t *testing.T) {
	l := NewRateLimiter(newBadger(t), kv.ErrNotFound)
	policy := sharedlimit.Policy{Limit: 1, Window: time.Hour}
	if decision := l.Allow("a", "1.1.1.1", policy); !decision.Allowed {
		t.Fatal("scope a, ip1: first rejected")
	}
	if decision := l.Allow("a", "1.1.1.1", policy); decision.Allowed {
		t.Fatal("scope a, ip1: second allowed, want rejected")
	}
	// A different scope and a different client each keep an independent budget.
	if decision := l.Allow("b", "1.1.1.1", policy); !decision.Allowed {
		t.Fatal("scope b, ip1: first rejected (independent counter)")
	}
	if decision := l.Allow("a", "2.2.2.2", policy); !decision.Allowed {
		t.Fatal("scope a, ip2: first rejected (independent counter)")
	}
}

func TestWindowResets(t *testing.T) {
	clock := time.Unix(1_700_000_000, 0)
	l := NewRateLimiter(newBadger(t), kv.ErrNotFound)
	l.now = func() time.Time { return clock }

	policy := sharedlimit.Policy{Limit: 1, Window: time.Minute}
	if decision := l.Allow("s", "ip", policy); !decision.Allowed {
		t.Fatal("first request rejected")
	}
	if decision := l.Allow("s", "ip", policy); decision.Allowed {
		t.Fatal("second request in same window allowed, want rejected")
	}
	// Advance past the window: a fresh counter key starts the budget over.
	clock = clock.Add(policy.Window + time.Second)
	if decision := l.Allow("s", "ip", policy); !decision.Allowed {
		t.Fatal("request after window reset rejected, want allowed")
	}
}

func TestSharedAcrossInstances(t *testing.T) {
	// Two limiters over ONE kv model two Cloud Run replicas: the limit must hold
	// across both, not be granted twice.
	store := newBadger(t)
	a := NewRateLimiter(store, kv.ErrNotFound)
	b := NewRateLimiter(store, kv.ErrNotFound)

	policy := sharedlimit.Policy{Limit: 4, Window: time.Hour}
	allowed := 0
	for i := 0; i < 10; i++ {
		l := a
		if i%2 == 1 {
			l = b
		}
		if decision := l.Allow("s", "ip", policy); decision.Allowed {
			allowed++
		}
	}
	if allowed != policy.Limit {
		t.Fatalf("shared limit: %d requests allowed across both instances, want %d", allowed, policy.Limit)
	}
}

func TestDisabledWhenNonPositive(t *testing.T) {
	l := NewRateLimiter(newBadger(t), kv.ErrNotFound)
	if decision := l.Allow("s", "ip", sharedlimit.Policy{Window: time.Hour}); !decision.Allowed {
		t.Fatal("limit 0 should disable limiting")
	}
	if decision := l.Allow("s", "ip", sharedlimit.Policy{Limit: 5}); !decision.Allowed {
		t.Fatal("window 0 should disable limiting")
	}
}
