package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// LogoutHandler must fail closed against CSRF: a request without a proven
// same-origin signal is rejected, and only a same-origin caller succeeds.
func TestLogoutFailsClosedOnCSRF(t *testing.T) {
	h, _, _ := testHandler(t)

	call := func(setHeaders func(r *http.Request)) int {
		r := gin.New()
		r.POST("/logout", h.LogoutHandler)
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/logout", nil)
		if setHeaders != nil {
			setHeaders(req)
		}
		r.ServeHTTP(w, req)
		return w.Code
	}

	// No Sec-Fetch-Site and no Origin/Referer: no same-origin signal -> reject.
	if code := call(nil); code != http.StatusForbidden {
		t.Fatalf("no signal: status = %d, want 403", code)
	}

	// No Sec-Fetch-Site, cross-origin Origin: host mismatch -> reject.
	if code := call(func(r *http.Request) {
		r.Header.Set("Origin", "https://evil.example")
	}); code != http.StatusForbidden {
		t.Fatalf("cross-origin: status = %d, want 403", code)
	}

	// Sec-Fetch-Site present but cross-site -> reject.
	if code := call(func(r *http.Request) {
		r.Header.Set("Sec-Fetch-Site", "cross-site")
	}); code != http.StatusForbidden {
		t.Fatalf("sec-fetch cross-site: status = %d, want 403", code)
	}

	// Sec-Fetch-Site same-origin -> accept.
	if code := call(func(r *http.Request) {
		r.Header.Set("Sec-Fetch-Site", "same-origin")
	}); code != http.StatusOK {
		t.Fatalf("same-origin: status = %d, want 200", code)
	}

	// Fallback: Origin matches the configured public origin -> accept.
	if code := call(func(r *http.Request) {
		r.Header.Set("Origin", "https://repo.example.com")
	}); code != http.StatusOK {
		t.Fatalf("configured origin match: status = %d, want 200", code)
	}
}
