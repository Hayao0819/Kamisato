package middleware

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/Hayao0819/Kamisato/ayato/repository/kv/badgerkv"
	"github.com/Hayao0819/Kamisato/internal/conf"
	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

func rlEngine(m *Middleware, limit rate.Limit, burst int) *gin.Engine {
	r := gin.New()
	r.GET("/p", m.RateLimit(limit, burst), func(c *gin.Context) { c.Status(http.StatusOK) })
	return r
}

func doReq(r *gin.Engine, remoteAddr string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/p", nil)
	req.RemoteAddr = remoteAddr
	r.ServeHTTP(w, req)
	return w
}

func newRLMiddleware(t *testing.T) *Middleware {
	t.Helper()
	gin.SetMode(gin.TestMode)
	store, err := badgerkv.New(t.TempDir())
	if err != nil {
		t.Fatalf("open badger: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return New(&conf.AyatoConfig{}).WithRateLimiter(store)
}

func TestRateLimitBurstThen429(t *testing.T) {
	m := newRLMiddleware(t)
	// rate is tiny so the burst is not replenished during the test.
	r := rlEngine(m, rate.Every(time.Hour), 3)

	const ip = "10.0.0.1:1234"
	for i := 0; i < 3; i++ {
		if w := doReq(r, ip); w.Code != http.StatusOK {
			t.Fatalf("request %d: status = %d, want 200", i+1, w.Code)
		}
	}
	w := doReq(r, ip)
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("4th request: status = %d, want 429", w.Code)
	}
	if w.Header().Get("Retry-After") == "" {
		t.Error("429 response missing Retry-After header")
	}
}

func TestRateLimitPerIP(t *testing.T) {
	m := newRLMiddleware(t)
	r := rlEngine(m, rate.Every(time.Hour), 1)

	if w := doReq(r, "10.0.0.1:1"); w.Code != http.StatusOK {
		t.Fatalf("ip1 first: status = %d, want 200", w.Code)
	}
	if w := doReq(r, "10.0.0.1:1"); w.Code != http.StatusTooManyRequests {
		t.Fatalf("ip1 second: status = %d, want 429", w.Code)
	}
	if w := doReq(r, "10.0.0.2:1"); w.Code != http.StatusOK {
		t.Fatalf("ip2 first: status = %d, want 200 (independent bucket)", w.Code)
	}
}

func TestRateLimitRefill(t *testing.T) {
	m := newRLMiddleware(t)
	// 50 tokens/sec -> ~20ms to refill one; burst 1.
	r := rlEngine(m, rate.Limit(50), 1)

	const ip = "10.0.0.3:1"
	if w := doReq(r, ip); w.Code != http.StatusOK {
		t.Fatalf("first: status = %d, want 200", w.Code)
	}
	if w := doReq(r, ip); w.Code != http.StatusTooManyRequests {
		t.Fatalf("immediate second: status = %d, want 429", w.Code)
	}
	// Poll until the bucket refills a token instead of sleeping a fixed span:
	// the limiter refills off the real clock, so waiting on the condition avoids
	// flaking on a slow scheduler.
	deadline := time.Now().Add(2 * time.Second)
	for {
		if w := doReq(r, ip); w.Code == http.StatusOK {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("token bucket did not refill within the deadline")
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestRateLimitConcurrent(t *testing.T) {
	m := newRLMiddleware(t)
	r := rlEngine(m, rate.Limit(1000), 1000)

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			ip := "10.1.0." + string(rune('0'+n%10)) + ":1"
			for j := 0; j < 20; j++ {
				doReq(r, ip)
			}
		}(i)
	}
	wg.Wait()
}
