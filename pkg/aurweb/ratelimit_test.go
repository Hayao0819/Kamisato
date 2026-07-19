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
