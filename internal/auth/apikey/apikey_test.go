package apikey

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestVerifierValidAndReject(t *testing.T) {
	// Empty keys are dropped; two live keys model a rotation window.
	v := NewVerifier([]string{"secret-a", "secret-b", ""})
	if !v.Enabled() {
		t.Fatal("Enabled() = false, want true with keys configured")
	}
	if !v.Valid("secret-a") || !v.Valid("secret-b") {
		t.Error("a configured key was rejected")
	}
	for _, bad := range []string{"", "secret-c", "secret-a ", "SECRET-A", "secret"} {
		if v.Valid(bad) {
			t.Errorf("Valid(%q) = true, want false", bad)
		}
	}
}

func TestVerifierDisabledWhenNoKeys(t *testing.T) {
	if NewVerifier(nil).Enabled() {
		t.Error("Enabled() = true, want false with no keys")
	}
	if NewVerifier([]string{"", ""}).Enabled() {
		t.Error("Enabled() = true, want false when only empty keys are given")
	}
}

func TestFromRequest(t *testing.T) {
	newCtx := func(set func(*http.Request)) *gin.Context {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		set(req)
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = req
		return c
	}
	cases := []struct {
		name string
		set  func(*http.Request)
		want string
	}{
		{"bearer is a user token, not an API key", func(r *http.Request) { r.Header.Set("Authorization", "Bearer tok123") }, ""},
		{"x-api-key", func(r *http.Request) { r.Header.Set("X-API-Key", "tok456") }, "tok456"},
		{"authorization does not shadow x-api-key", func(r *http.Request) {
			r.Header.Set("Authorization", "Bearer user-token")
			r.Header.Set("X-API-Key", "tok789")
		}, "tok789"},
		{"none", func(r *http.Request) {}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := FromRequest(newCtx(tc.set)); got != tc.want {
				t.Errorf("FromRequest = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)
	run := func(v *Verifier, setKey func(*http.Request)) int {
		r := gin.New()
		r.GET("/", v.Middleware(), func(c *gin.Context) { c.Status(http.StatusOK) })
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		setKey(req)
		r.ServeHTTP(w, req)
		return w.Code
	}

	v := NewVerifier([]string{"good"})
	if code := run(v, func(*http.Request) {}); code != http.StatusUnauthorized {
		t.Errorf("no key: status = %d, want 401", code)
	}
	if code := run(v, func(r *http.Request) { r.Header.Set("X-API-Key", "good") }); code != http.StatusOK {
		t.Errorf("valid key: status = %d, want 200", code)
	}
	if code := run(v, func(r *http.Request) { r.Header.Set("Authorization", "Bearer good") }); code != http.StatusUnauthorized {
		t.Errorf("Bearer key: status = %d, want 401", code)
	}
	if code := run(v, func(r *http.Request) { r.Header.Set("X-API-Key", "bad") }); code != http.StatusUnauthorized {
		t.Errorf("bad key: status = %d, want 401", code)
	}

	// With no keys configured the middleware passes through (closed-network trust).
	if code := run(NewVerifier(nil), func(*http.Request) {}); code != http.StatusOK {
		t.Errorf("open middleware: status = %d, want 200", code)
	}
}

func TestScopedMiddlewareAndPrincipal(t *testing.T) {
	gin.SetMode(gin.TestMode)
	verifier := NewScopedVerifier([]Entry{
		{Name: "thoma", Key: "direct", Scopes: []string{ScopeBuildSubmit, ScopeBuildRead}},
		{Name: "ayato", Key: "proxy", Scopes: []string{ScopeBuildAdmin}},
	})
	run := func(key, scope string) (int, string) {
		engine := gin.New()
		name := ""
		engine.GET("/", verifier.Middleware(scope), func(c *gin.Context) {
			if principal, ok := PrincipalFrom(c); ok {
				name = principal.Name
			}
			c.Status(http.StatusOK)
		})
		request := httptest.NewRequest(http.MethodGet, "/", nil)
		request.Header.Set(Header, key)
		response := httptest.NewRecorder()
		engine.ServeHTTP(response, request)
		return response.Code, name
	}
	if code, name := run("direct", ScopeBuildRead); code != http.StatusOK || name != "thoma" {
		t.Fatalf("read = status %d principal %q", code, name)
	}
	if code, _ := run("direct", ScopeBuildCancel); code != http.StatusForbidden {
		t.Fatalf("unscoped cancel = status %d, want 403", code)
	}
	if code, name := run("proxy", ScopeBuildSubmit); code != http.StatusOK || name != "ayato" {
		t.Fatalf("build admin = status %d principal %q", code, name)
	}
}

func TestRotatedKeysShareStablePrincipalButKeepDistinctKeyIDs(t *testing.T) {
	verifier := NewScopedVerifier([]Entry{
		{Name: "thoma-2026-01", Principal: "thoma-builder", Key: "old", Scopes: []string{ScopeBuildRead}},
		{Name: "thoma-2026-07", Principal: "thoma-builder", Key: "new", Scopes: []string{ScopeBuildRead}},
	})
	for key, keyID := range map[string]string{"old": "thoma-2026-01", "new": "thoma-2026-07"} {
		principal, ok := verifier.Authenticate(key)
		if !ok {
			t.Fatalf("key %q did not authenticate", keyID)
		}
		if principal.Name != "thoma-builder" || principal.KeyID != keyID {
			t.Fatalf("key %q principal = %#v", keyID, principal)
		}
	}
}
