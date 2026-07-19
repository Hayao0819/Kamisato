package ratelimit

import (
	"testing"
	"time"
)

func TestMemoryLimitIsolationAndReset(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	limiter := NewMemory(10)
	limiter.now = func() time.Time { return now }
	policy := Policy{Limit: 2, Window: time.Minute}

	for request := 1; request <= policy.Limit; request++ {
		if decision := limiter.Allow("rpc", "client-a", policy); !decision.Allowed {
			t.Fatalf("request %d was rejected: %+v", request, decision)
		}
	}
	rejected := limiter.Allow("rpc", "client-a", policy)
	if rejected.Allowed {
		t.Fatal("over-limit request was allowed")
	}
	if rejected.RetryAfter <= 0 || rejected.RetryAfter > policy.Window {
		t.Fatalf("RetryAfter = %v, want within (0, %v]", rejected.RetryAfter, policy.Window)
	}

	for _, key := range []struct {
		scope  string
		client string
	}{
		{scope: "other", client: "client-a"},
		{scope: "rpc", client: "client-b"},
	} {
		if decision := limiter.Allow(key.scope, key.client, policy); !decision.Allowed {
			t.Errorf("%s/%s did not have an independent budget", key.scope, key.client)
		}
	}

	now = now.Add(policy.Window)
	if decision := limiter.Allow("rpc", "client-a", policy); !decision.Allowed {
		t.Error("request after the window reset was rejected")
	}
}

func TestMemoryBoundsDistinctClients(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	limiter := NewMemory(2)
	limiter.now = func() time.Time { return now }
	policy := Policy{Limit: 1, Window: time.Hour}

	limiter.Allow("rpc", "oldest", policy)
	now = now.Add(time.Second)
	limiter.Allow("rpc", "newer", policy)
	now = now.Add(time.Second)
	limiter.Allow("rpc", "newest", policy)

	if got := len(limiter.buckets); got != 2 {
		t.Fatalf("bucket count = %d, want 2", got)
	}
	if decision := limiter.Allow("rpc", "oldest", policy); !decision.Allowed {
		t.Error("least-recent bucket was not evicted")
	}
}

func TestDisabledPolicyAlwaysAllows(t *testing.T) {
	limiter := NewMemory(1)
	for _, policy := range []Policy{
		{},
		{Limit: 1},
		{Window: time.Minute},
	} {
		if decision := limiter.Allow("scope", "client", policy); !decision.Allowed {
			t.Errorf("disabled policy %+v rejected a request", policy)
		}
	}
}

func TestRetryAfterValue(t *testing.T) {
	for _, test := range []struct {
		retry time.Duration
		want  string
	}{
		{retry: 0, want: "1"},
		{retry: time.Millisecond, want: "1"},
		{retry: time.Second, want: "1"},
		{retry: time.Second + time.Millisecond, want: "2"},
	} {
		if got := RetryAfterValue(test.retry); got != test.want {
			t.Errorf("RetryAfterValue(%v) = %q, want %q", test.retry, got, test.want)
		}
	}
}
