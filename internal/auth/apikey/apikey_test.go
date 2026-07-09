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
		{"bearer", func(r *http.Request) { r.Header.Set("Authorization", "Bearer tok123") }, "tok123"},
		{"x-api-key", func(r *http.Request) { r.Header.Set("X-API-Key", "tok456") }, "tok456"},
		{"non-bearer auth falls through to x-api-key", func(r *http.Request) {
			r.Header.Set("Authorization", "Basic abc")
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
	if code := run(v, func(r *http.Request) { r.Header.Set("Authorization", "Bearer good") }); code != http.StatusOK {
		t.Errorf("valid key: status = %d, want 200", code)
	}
	if code := run(v, func(r *http.Request) { r.Header.Set("X-API-Key", "bad") }); code != http.StatusUnauthorized {
		t.Errorf("bad key: status = %d, want 401", code)
	}

	// With no keys configured the middleware passes through (closed-network trust).
	if code := run(NewVerifier(nil), func(*http.Request) {}); code != http.StatusOK {
		t.Errorf("open middleware: status = %d, want 200", code)
	}
}
