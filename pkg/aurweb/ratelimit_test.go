package aurweb

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/synctest"
	"time"
)

func TestRPCDocPage(t *testing.T) {
	s := newTestServer()
	for _, target := range []string{"/rpc", "/rpc/", "/rpc.php"} {
		req := httptest.NewRequest(http.MethodGet, target, nil)
		rec := httptest.NewRecorder()
		s.RPC(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("%s: status = %d, want 200", target, rec.Code)
		}
		if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
			t.Errorf("%s: Content-Type = %q, want text/html", target, ct)
		}
		if !strings.Contains(rec.Body.String(), "RPC Interface") {
			t.Errorf("%s: body is not the doc page", target)
		}
	}

	// A query-bearing request is still handled as an API call, not the doc page.
	req := httptest.NewRequest(http.MethodGet, "/rpc?v=5&type=search&arg=mytool", nil)
	rec := httptest.NewRecorder()
	s.RPC(rec, req)
	if ct := rec.Header().Get("Content-Type"); strings.HasPrefix(ct, "text/html") {
		t.Error("a query request should not get the HTML doc page")
	}
}

func TestRateLimit(t *testing.T) {
	be := &stubBackend{pkgs: map[string]Pkg{"x": {Name: "x", PackageBase: "x", Version: "1"}}}
	s := New(be, WithRateLimit(2, time.Hour, nil))

	do := func() int {
		req := httptest.NewRequest(http.MethodGet, "/rpc?v=5&type=search&arg=x", nil)
		req.RemoteAddr = "10.0.0.1:1234"
		rec := httptest.NewRecorder()
		s.RPC(rec, req)
		return rec.Code
	}
	if c := do(); c != http.StatusOK {
		t.Fatalf("req 1 = %d, want 200", c)
	}
	if c := do(); c != http.StatusOK {
		t.Fatalf("req 2 = %d, want 200", c)
	}
	if c := do(); c != http.StatusTooManyRequests {
		t.Fatalf("req 3 = %d, want 429", c)
	}
	req := httptest.NewRequest(http.MethodGet, "/rpc?v=5&type=search&arg=x", nil)
	req.RemoteAddr = "10.0.0.1:1234"
	rec := httptest.NewRecorder()
	s.RPC(rec, req)
	if rec.Header().Get("Retry-After") == "" {
		t.Error("429 response is missing Retry-After")
	}

	// A different client has its own bucket.
	req = httptest.NewRequest(http.MethodGet, "/rpc?v=5&type=search&arg=x", nil)
	req.RemoteAddr = "10.0.0.2:1234"
	rec = httptest.NewRecorder()
	s.RPC(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("second client = %d, want 200 (independent bucket)", rec.Code)
	}
}

func TestRateLimitWindowReset(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		be := &stubBackend{pkgs: map[string]Pkg{"x": {Name: "x", PackageBase: "x", Version: "1"}}}
		s := New(be, WithRateLimit(1, 50*time.Millisecond, nil))
		do := func() int {
			req := httptest.NewRequest(http.MethodGet, "/rpc?v=5&type=search&arg=x", nil)
			rec := httptest.NewRecorder()
			s.RPC(rec, req)
			return rec.Code
		}
		if status := do(); status != http.StatusOK {
			t.Fatalf("first request = %d, want 200", status)
		}
		if status := do(); status != http.StatusTooManyRequests {
			t.Fatalf("second request = %d, want 429", status)
		}
		time.Sleep(70 * time.Millisecond)
		if status := do(); status != http.StatusOK {
			t.Errorf("request after window = %d, want 200", status)
		}
	})
}

func TestMemoryRateLimiterBoundsClients(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	limiter := newMemoryRateLimiter(2)
	limiter.now = func() time.Time { return now }

	limiter.allow("oldest", 1, time.Hour)
	now = now.Add(time.Second)
	limiter.allow("newer", 1, time.Hour)
	now = now.Add(time.Second)
	limiter.allow("newest", 1, time.Hour)

	if len(limiter.buckets) != 2 {
		t.Fatalf("client count = %d, want 2", len(limiter.buckets))
	}
	if allowed, _ := limiter.allow("oldest", 1, time.Hour); !allowed {
		t.Fatal("oldest client was not evicted")
	}
}

func TestMemoryRateLimiterDisabled(t *testing.T) {
	limiter := newMemoryRateLimiter(1)
	if allowed, _ := limiter.allow("client", 0, time.Minute); !allowed {
		t.Fatal("zero limit must disable limiting")
	}
	if allowed, _ := limiter.allow("client", 1, 0); !allowed {
		t.Fatal("zero window must disable limiting")
	}
}

func TestRetryAfterValue(t *testing.T) {
	tests := map[time.Duration]string{
		0:                              "1",
		time.Millisecond:               "1",
		time.Second:                    "1",
		time.Second + time.Millisecond: "2",
	}
	for retry, want := range tests {
		if got := retryAfterValue(retry); got != want {
			t.Errorf("retryAfterValue(%v) = %q, want %q", retry, got, want)
		}
	}
}
