package handler

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/Hayao0819/Kamisato/internal/conf"
)

type proxyRoundTripFunc func(*http.Request) (*http.Response, error)

func (f proxyRoundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

type closeNotifyRecorder struct{ *httptest.ResponseRecorder }

func (r *closeNotifyRecorder) CloseNotify() <-chan bool {
	return make(chan bool)
}

// The Director must strip the client's Authorization and X-API-Key so a CLI
// token never leaks to miko.
func TestMikoProxyStripsAuthorization(t *testing.T) {
	cfg := &conf.AyatoConfig{}
	cfg.Miko.URL = "http://miko.internal:8081"
	cfg.Miko.APIKey = "shared-secret"

	mp, err := NewMikoProxy(cfg)
	if err != nil {
		t.Fatalf("NewMikoProxy: %v", err)
	}
	if mp == nil {
		t.Fatalf("expected a proxy, got nil")
	}

	req := httptest.NewRequest(http.MethodPost, "http://ayato/api/unstable/build", nil)
	req.Header.Set("Authorization", "Bearer user-cli-token")
	req.Header.Set("X-API-Key", "client-supplied-key")
	req.Header.Set("Cookie", "ayato_session=secret-sid")
	req.Header.Set("X-Log-Token", "one-time-user-token")

	mp.proxy.Director(req)

	if got := req.Header.Get("Authorization"); got != "" {
		t.Fatalf("Authorization must be stripped, got %q", got)
	}
	if got := req.Header.Get("Cookie"); got != "" {
		t.Fatalf("Cookie must be stripped (no user session into miko), got %q", got)
	}
	if got := req.Header.Get("X-Log-Token"); got != "" {
		t.Fatalf("X-Log-Token must be stripped, got %q", got)
	}
	if got := req.Header.Get("X-API-Key"); got != "shared-secret" {
		t.Fatalf("X-API-Key must be the shared secret, got %q", got)
	}
}

func TestMikoProxyUsesSharedPrefixAndSegmentEscaping(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := &conf.AyatoConfig{}
	cfg.Miko.URL = "https://miko.internal/base/prefix"
	cfg.Miko.APIKey = "proxy-key"
	proxy, err := NewMikoProxy(cfg)
	if err != nil {
		t.Fatal(err)
	}

	var gotPath, gotKey, gotAuthorization string
	proxy.proxy.Transport = proxyRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		gotPath = request.URL.EscapedPath()
		gotKey = request.Header.Get("X-API-Key")
		gotAuthorization = request.Header.Get("Authorization")
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{}`)),
		}, nil
	})

	router := gin.New()
	router.GET("/jobs/:id", proxy.HandlerFunc(func(c *gin.Context) []string {
		return []string{"api", "unstable", "jobs", c.Param("id")}
	}))
	response := &closeNotifyRecorder{ResponseRecorder: httptest.NewRecorder()}
	request := httptest.NewRequest(http.MethodGet, "/jobs/a%20b", nil)
	request.Header.Set("Authorization", "Bearer user-secret")
	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d", response.Code)
	}
	if gotPath != "/base/prefix/api/unstable/jobs/a%20b" {
		t.Fatalf("upstream path = %q", gotPath)
	}
	if gotKey != "proxy-key" || gotAuthorization != "" {
		t.Fatalf("upstream credentials = X-API-Key %q, Authorization %q", gotKey, gotAuthorization)
	}
}

func TestMikoProxyRejectsCredentialBearingURL(t *testing.T) {
	cfg := &conf.AyatoConfig{}
	cfg.Miko.URL = "https://user:password@miko.internal"
	if _, err := NewMikoProxy(cfg); err == nil {
		t.Fatal("credential-bearing Miko URL unexpectedly accepted")
	}
}
