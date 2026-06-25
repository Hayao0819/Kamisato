package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Hayao0819/Kamisato/ayato/auth"
	"github.com/Hayao0819/Kamisato/ayato/kv/badgerkv"
	"github.com/Hayao0819/Kamisato/internal/conf"
	"github.com/gin-gonic/gin"
)

const testSecret = "0123456789abcdef0123456789abcdef" // 32 bytes

func testMiddleware(t *testing.T, bootstrap int64) (*Middleware, *auth.AllowlistRepo, *auth.Signer) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	store, err := badgerkv.New(t.TempDir())
	if err != nil {
		t.Fatalf("badgerkv.New: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	allow := auth.NewAllowlistRepo(store)
	if err := auth.SeedBootstrap(allow, bootstrap); err != nil {
		t.Fatalf("SeedBootstrap: %v", err)
	}
	signer, err := auth.NewSigner([]string{testSecret})
	if err != nil {
		t.Fatalf("NewSigner: %v", err)
	}
	cfg := &conf.AyatoConfig{}
	m := New(cfg).WithAuth(allow, signer)
	return m, allow, signer
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

func TestRequireAdminNoCredentials(t *testing.T) {
	m, _, _ := testMiddleware(t, 42)
	w := run(m, false, func(*http.Request) {})
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("no creds: status = %d, want 401", w.Code)
	}
}

func TestRequireAdminBearerToken(t *testing.T) {
	m, _, signer := testMiddleware(t, 42)
	tok := cliToken(t, signer, 42, "alice")

	w := run(m, false, func(req *http.Request) {
		req.Header.Set("Authorization", "Bearer "+tok)
	})
	if w.Code != http.StatusOK {
		t.Fatalf("valid bearer of allowlisted id: status = %d, want 200", w.Code)
	}
}

func TestRequireAdminBearerNotAllowlisted(t *testing.T) {
	m, _, signer := testMiddleware(t, 42)
	// Signed token for an id that is NOT on the allowlist: the per-request
	// allow.Has re-check must reject it (the de-allowlist path).
	tok := cliToken(t, signer, 99, "mallory")

	w := run(m, false, func(req *http.Request) {
		req.Header.Set("Authorization", "Bearer "+tok)
	})
	if w.Code != http.StatusForbidden {
		t.Fatalf("bearer of non-allowlisted id: status = %d, want 403", w.Code)
	}
}

func TestRequireAdminBearerWrongType(t *testing.T) {
	m, _, signer := testMiddleware(t, 42)
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
	m, _, signer := testMiddleware(t, 42)
	sid := sessionToken(t, signer, 42, "alice")

	// Without Sec-Fetch-Site: same-origin the cookie path is rejected (CSRF).
	w := run(m, false, func(req *http.Request) {
		req.AddCookie(&http.Cookie{Name: m.cfg.Auth.CookieName(), Value: sid})
	})
	if w.Code != http.StatusForbidden {
		t.Fatalf("cookie without Sec-Fetch-Site: status = %d, want 403", w.Code)
	}

	// With the header set, the same cookie is accepted.
	w = run(m, false, func(req *http.Request) {
		req.AddCookie(&http.Cookie{Name: m.cfg.Auth.CookieName(), Value: sid})
		req.Header.Set("Sec-Fetch-Site", "same-origin")
	})
	if w.Code != http.StatusOK {
		t.Fatalf("cookie with Sec-Fetch-Site same-origin: status = %d, want 200", w.Code)
	}
}

func TestRequireAdminBasicTokenBlinkyOnly(t *testing.T) {
	m, _, signer := testMiddleware(t, 42)
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
