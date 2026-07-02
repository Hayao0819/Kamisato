package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Hayao0819/Kamisato/ayato/auth"
	"github.com/Hayao0819/Kamisato/internal/conf"
	"github.com/gin-gonic/gin"
)

const testSecret = "0123456789abcdef0123456789abcdef" // 32 bytes

// fakeChecker is a minimal adminChecker backed by a static allowlist.
type fakeChecker struct {
	allowed map[int64]bool
}

func (f fakeChecker) IsAdmin(id int64) bool {
	if id <= 0 {
		return false
	}
	return f.allowed[id]
}

// fakeDenylist is a minimal denylistChecker backed by a static revoked set.
type fakeDenylist struct{ revoked map[string]bool }

func (f fakeDenylist) IsRevoked(jti string) bool { return f.revoked[jti] }

func testMiddleware(t *testing.T, bootstrap int64) (*Middleware, *auth.Signer) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	checker := fakeChecker{allowed: map[int64]bool{}}
	if bootstrap > 0 {
		checker.allowed[bootstrap] = true
	}
	signer, err := auth.NewSigner([]string{testSecret})
	if err != nil {
		t.Fatalf("NewSigner: %v", err)
	}
	cfg := &conf.AyatoConfig{}
	m := New(cfg).WithAuth(checker, signer)
	return m, signer
}

func sessionToken(t *testing.T, s *auth.Signer, id int64, login string) string {
	t.Helper()
	tok, err := s.Sign(auth.Claims{Typ: auth.TypSession, GitHubID: id, Login: login, Exp: time.Now().Add(time.Hour)})
	if err != nil {
		t.Fatalf("sign session: %v", err)
	}
	return tok
}

func cliToken(t *testing.T, s *auth.Signer, id int64, login string) string {
	t.Helper()
	tok, err := s.Sign(auth.Claims{Typ: auth.TypCLI, GitHubID: id, Login: login, Name: "cli", Exp: time.Now().Add(time.Hour)})
	if err != nil {
		t.Fatalf("sign cli: %v", err)
	}
	return tok
}

func run(m *Middleware, allowBasic bool, mutate func(*http.Request)) *httptest.ResponseRecorder {
	r := gin.New()
	r.GET("/p", m.RequireAdmin(allowBasic), func(c *gin.Context) { c.Status(http.StatusOK) })
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/p", nil)
	mutate(req)
	r.ServeHTTP(w, req)
	return w
}

// Without WithAuth there is no checker/signer, so RequireAdmin must fail closed (503).
func TestRequireAdminFailsClosedWithoutAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	m := New(&conf.AyatoConfig{})
	for _, allowBasic := range []bool{true, false} {
		w := run(m, allowBasic, func(*http.Request) {})
		if w.Code != http.StatusServiceUnavailable {
			t.Fatalf("allowBasic=%v: status = %d, want 503 (fail closed when auth is unconfigured)", allowBasic, w.Code)
		}
	}
}

func TestRequireAdminNoCredentials(t *testing.T) {
	m, _ := testMiddleware(t, 42)
	w := run(m, false, func(*http.Request) {})
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("no creds: status = %d, want 401", w.Code)
	}
}

func TestRequireAdminBearerToken(t *testing.T) {
	m, signer := testMiddleware(t, 42)
	tok := cliToken(t, signer, 42, "alice")

	w := run(m, false, func(req *http.Request) {
		req.Header.Set("Authorization", "Bearer "+tok)
	})
	if w.Code != http.StatusOK {
		t.Fatalf("valid bearer of allowlisted id: status = %d, want 200", w.Code)
	}
}

func TestRequireAdminBearerNotAllowlisted(t *testing.T) {
	m, signer := testMiddleware(t, 42)
	// Signed token for an id NOT on the allowlist: the per-request IsAdmin
	// re-check must reject it (the de-allowlist path).
	tok := cliToken(t, signer, 99, "mallory")

	w := run(m, false, func(req *http.Request) {
		req.Header.Set("Authorization", "Bearer "+tok)
	})
	if w.Code != http.StatusForbidden {
		t.Fatalf("bearer of non-allowlisted id: status = %d, want 403", w.Code)
	}
}

func TestRequireAdminBearerWrongType(t *testing.T) {
	m, signer := testMiddleware(t, 42)
	// A session-typed token must not authenticate the Bearer path (type pinning).
	tok := sessionToken(t, signer, 42, "alice")

	w := run(m, false, func(req *http.Request) {
		req.Header.Set("Authorization", "Bearer "+tok)
	})
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("session token on bearer path: status = %d, want 401", w.Code)
	}
}

func TestRequireAdminCookieRequiresSecFetch(t *testing.T) {
	m, signer := testMiddleware(t, 42)
	sid := sessionToken(t, signer, 42, "alice")

	// Without Sec-Fetch-Site: same-origin the cookie path is rejected (CSRF).
	w := run(m, false, func(req *http.Request) {
		req.AddCookie(&http.Cookie{Name: m.cfg.Auth.CookieName(), Value: sid})
	})
	if w.Code != http.StatusForbidden {
		t.Fatalf("cookie without Sec-Fetch-Site: status = %d, want 403", w.Code)
	}

	w = run(m, false, func(req *http.Request) {
		req.AddCookie(&http.Cookie{Name: m.cfg.Auth.CookieName(), Value: sid})
		req.Header.Set("Sec-Fetch-Site", "same-origin")
	})
	if w.Code != http.StatusOK {
		t.Fatalf("cookie with Sec-Fetch-Site same-origin: status = %d, want 200", w.Code)
	}
}

// When the browser omits Sec-Fetch-Site, the cookie path falls back to an
// Origin/Referer allowlist: a matching origin passes, a foreign one is rejected,
// and no resolvable origin fails closed.
func TestRequireAdminCookieOriginFallback(t *testing.T) {
	m, signer := testMiddleware(t, 42)
	m.cfg.Auth.PublicOrigin = "https://repo.example.com"
	sid := sessionToken(t, signer, 42, "alice")

	withCookie := func(mutate func(*http.Request)) *httptest.ResponseRecorder {
		return run(m, false, func(req *http.Request) {
			req.AddCookie(&http.Cookie{Name: m.cfg.Auth.CookieName(), Value: sid})
			mutate(req)
		})
	}

	if w := withCookie(func(req *http.Request) {
		req.Header.Set("Origin", "https://repo.example.com")
	}); w.Code != http.StatusOK {
		t.Fatalf("matching Origin without Sec-Fetch-Site: status = %d, want 200", w.Code)
	}
	if w := withCookie(func(req *http.Request) {
		req.Header.Set("Origin", "https://evil.example.com")
	}); w.Code != http.StatusForbidden {
		t.Fatalf("foreign Origin: status = %d, want 403", w.Code)
	}
	if w := withCookie(func(req *http.Request) {
		req.Header.Set("Referer", "https://repo.example.com/packages")
	}); w.Code != http.StatusOK {
		t.Fatalf("matching Referer without Origin: status = %d, want 200", w.Code)
	}
	if w := withCookie(func(*http.Request) {}); w.Code != http.StatusForbidden {
		t.Fatalf("no origin headers: status = %d, want 403 (fail closed)", w.Code)
	}
}

// A CLI token whose jti is on the denylist must fail the Bearer path (401),
// while an otherwise-identical token with a live jti still authenticates.
func TestRequireAdminRevokedTokenRejected(t *testing.T) {
	m, signer := testMiddleware(t, 42)
	m.WithDenylist(fakeDenylist{revoked: map[string]bool{"revoked-jti": true}})

	revoked, err := signer.Sign(auth.Claims{Typ: auth.TypCLI, GitHubID: 42, Login: "alice", Name: "cli", JTI: "revoked-jti", Exp: time.Now().Add(time.Hour)})
	if err != nil {
		t.Fatalf("sign revoked: %v", err)
	}
	if w := run(m, false, func(req *http.Request) { req.Header.Set("Authorization", "Bearer "+revoked) }); w.Code != http.StatusUnauthorized {
		t.Fatalf("revoked token: status = %d, want 401", w.Code)
	}

	live, err := signer.Sign(auth.Claims{Typ: auth.TypCLI, GitHubID: 42, Login: "alice", Name: "cli", JTI: "live-jti", Exp: time.Now().Add(time.Hour)})
	if err != nil {
		t.Fatalf("sign live: %v", err)
	}
	if w := run(m, false, func(req *http.Request) { req.Header.Set("Authorization", "Bearer "+live) }); w.Code != http.StatusOK {
		t.Fatalf("non-revoked token: status = %d, want 200", w.Code)
	}
}

// An access token past its exp is rejected (401), and the response carries the
// refresh hint header so the CLI knows to refresh instead of prompting a re-login.
// A garbage token gets a plain 401 with no hint.
func TestRequireAdminExpiredAccessTokenHint(t *testing.T) {
	m, signer := testMiddleware(t, 42)
	expired, err := signer.Sign(auth.Claims{Typ: auth.TypCLI, GitHubID: 42, Login: "alice", Name: "cli", JTI: "a-old", Exp: time.Now().Add(-time.Minute)})
	if err != nil {
		t.Fatalf("sign expired: %v", err)
	}

	w := run(m, false, func(req *http.Request) { req.Header.Set("Authorization", "Bearer "+expired) })
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expired access: status = %d, want 401", w.Code)
	}
	if w.Header().Get("X-Access-Token-Expired") != "1" {
		t.Fatal("an expired access token must set the refresh hint header")
	}

	// A malformed token is a plain 401 without the refresh hint.
	w = run(m, false, func(req *http.Request) { req.Header.Set("Authorization", "Bearer not-a-token") })
	if w.Code != http.StatusUnauthorized || w.Header().Get("X-Access-Token-Expired") == "1" {
		t.Fatalf("garbage token: status = %d hint = %q, want 401 with no hint", w.Code, w.Header().Get("X-Access-Token-Expired"))
	}
}

func TestRequireAdminBasicTokenBlinkyOnly(t *testing.T) {
	m, signer := testMiddleware(t, 42)
	tok := cliToken(t, signer, 42, "alice")

	basic := func(req *http.Request) {
		// Username ignored; password carries the signed CLI token.
		req.SetBasicAuth("anything", tok)
	}

	// allowBasic=true (blinky routes): accepted.
	if w := run(m, true, basic); w.Code != http.StatusOK {
		t.Fatalf("basic token with allowBasic=true: status = %d, want 200", w.Code)
	}
	// allowBasic=false (miko/admin routes): Basic not consulted -> 401.
	if w := run(m, false, basic); w.Code != http.StatusUnauthorized {
		t.Fatalf("basic token with allowBasic=false: status = %d, want 401", w.Code)
	}
}
