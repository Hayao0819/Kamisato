package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

// rlEngineTrusted mirrors production wiring (ayato/cmd/root.go): trust NONE by
// default, set the configured CIDR only when given. That is what makes
// c.ClientIP() return the real peer.
func rlEngineTrusted(t *testing.T, m *Middleware, limit rate.Limit, burst int, trusted []string) *gin.Engine {
	t.Helper()
	r := gin.New()
	if err := r.SetTrustedProxies(nil); err != nil {
		t.Fatalf("SetTrustedProxies(nil): %v", err)
	}
	if len(trusted) > 0 {
		if err := r.SetTrustedProxies(trusted); err != nil {
			t.Fatalf("SetTrustedProxies(%v): %v", trusted, err)
		}
	}
	r.GET("/p", m.RateLimit(limit, burst), func(c *gin.Context) { c.Status(http.StatusOK) })
	return r
}

func doReqXFF(r *gin.Engine, remoteAddr, xff string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/p", nil)
	req.RemoteAddr = remoteAddr
	if xff != "" {
		req.Header.Set("X-Forwarded-For", xff)
	}
	r.ServeHTTP(w, req)
	return w
}

// TestRedteam_RotatingXFFNoBypass re-runs the prior rotating-X-Forwarded-For
// bypass against the production wiring (SetTrustedProxies(nil)). A single peer
// rotates a fresh forged XFF on every request; with XFF ignored, all requests
// key on the SAME RemoteAddr and the limiter must 429 after the burst.
func TestRedteam_RotatingXFFNoBypass(t *testing.T) {
	m := newRLMiddleware(t)
	// tiny rate so the burst is never replenished mid-test; burst 3.
	r := rlEngineTrusted(t, m, rate.Every(time.Hour), 3, nil)

	const peer = "203.0.113.7:55000" // the real attacker peer, fixed
	// First 3 (burst) succeed regardless of the forged header.
	for i := 0; i < 3; i++ {
		xff := rotating(i)
		if w := doReqXFF(r, peer, xff); w.Code != http.StatusOK {
			t.Fatalf("burst req %d (xff=%s): status=%d want 200", i+1, xff, w.Code)
		}
	}
	// Every subsequent request, with a brand-new forged XFF each time, must 429:
	// the key is the real peer, so the bucket is exhausted.
	for i := 3; i < 25; i++ {
		xff := rotating(i)
		w := doReqXFF(r, peer, xff)
		if w.Code != http.StatusTooManyRequests {
			t.Fatalf("rotating-XFF req %d (xff=%s) BYPASSED rate limit: status=%d want 429", i+1, xff, w.Code)
		}
	}
}

func rotating(i int) string {
	// fresh, syntactically-valid public IP per request
	return "198.51.100." + itoa(i%200+1)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [3]byte
	p := len(b)
	for n > 0 {
		p--
		b[p] = byte('0' + n%10)
		n /= 10
	}
	return string(b[p:])
}

// TestRedteam_XFFHonoredOnlyFromTrustedCIDR confirms that when trusted_proxies
// IS set, a forged XFF is honored only when the immediate peer is inside that
// CIDR. A peer OUTSIDE the CIDR cannot inject a client IP (its XFF is ignored,
// key stays the real peer); a peer INSIDE the CIDR has its rightmost-untrusted
// XFF hop used as the key.
func TestRedteam_XFFHonoredOnlyFromTrustedCIDR(t *testing.T) {
	m := newRLMiddleware(t)
	r := rlEngineTrusted(t, m, rate.Every(time.Hour), 1, []string{"10.0.0.0/8"})

	// (a) Untrusted peer rotating XFF: must NOT bypass (key = real peer).
	const untrusted = "203.0.113.9:40000"
	if w := doReqXFF(r, untrusted, "198.51.100.1"); w.Code != http.StatusOK {
		t.Fatalf("untrusted peer first: status=%d want 200", w.Code)
	}
	if w := doReqXFF(r, untrusted, "198.51.100.2"); w.Code != http.StatusTooManyRequests {
		t.Fatalf("untrusted peer rotated XFF BYPASSED limit: status=%d want 429", w.Code)
	}

	// (b) Trusted proxy peer forwarding distinct real clients: each distinct
	// client IP gets its own bucket (legitimate per-client limiting).
	const proxy = "10.1.2.3:1"
	if w := doReqXFF(r, proxy, "192.0.2.50"); w.Code != http.StatusOK {
		t.Fatalf("trusted proxy client A first: status=%d want 200", w.Code)
	}
	if w := doReqXFF(r, proxy, "192.0.2.50"); w.Code != http.StatusTooManyRequests {
		t.Fatalf("trusted proxy client A second: status=%d want 429", w.Code)
	}
	if w := doReqXFF(r, proxy, "192.0.2.51"); w.Code != http.StatusOK {
		t.Fatalf("trusted proxy client B first: status=%d want 200 (independent bucket)", w.Code)
	}
}

// TestRedteam_TrustAllSpellingHonorsForgedXFF is the runtime half of the
// conf.TestRedteam_TrustAllSpellingBypass PoC: it proves that the any-net
// spellings Validate fails to catch ("0000:0000::/0", "0.0.0.0/00") cause gin to
// honor a forged X-Forwarded-For from an ARBITRARY peer, which re-enables the
// rotating-XFF rate-limit bypass. The canonical forms ("::/0","0.0.0.0/0") behave
// identically — they are only blocked because Validate string-matches them.
func TestRedteam_TrustAllSpellingHonorsForgedXFF(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cases := []struct {
		cidr, peer        string
		blockedByValidate bool
	}{
		{"0000:0000::/0", "[2001:db8::dead]:9999", false}, // PoC: slips past Validate
		{"0.0.0.0/00", "203.0.113.9:5000", false},         // PoC: slips past Validate
		{"::/0", "[2001:db8::dead]:9999", true},           // canonical: Validate blocks it
		{"0.0.0.0/0", "203.0.113.9:5000", true},           // canonical: Validate blocks it
	}
	for _, tc := range cases {
		r := gin.New()
		if err := r.SetTrustedProxies(nil); err != nil {
			t.Fatalf("reset: %v", err)
		}
		if err := r.SetTrustedProxies([]string{tc.cidr}); err != nil {
			t.Logf("%q rejected by gin at startup (fail-closed): %v", tc.cidr, err)
			continue
		}
		var got string
		r.GET("/", func(c *gin.Context) { got = c.ClientIP(); c.Status(http.StatusOK) })
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = tc.peer
		req.Header.Set("X-Forwarded-For", "1.2.3.4")
		r.ServeHTTP(w, req)
		honored := got == "1.2.3.4"
		if !honored {
			t.Fatalf("%q: expected gin to honor forged XFF (trust-all), but ClientIP=%q", tc.cidr, got)
		}
		if !tc.blockedByValidate {
			t.Logf("VULN: %q passes conf.Validate AND makes gin trust forged XFF from %s -> bypass", tc.cidr, tc.peer)
		} else {
			t.Logf("(canonical) %q makes gin trust forged XFF, but conf.Validate rejects this spelling", tc.cidr)
		}
	}
}
