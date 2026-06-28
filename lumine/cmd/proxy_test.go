package cmd

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// The proxy must replace spoofed forwarding headers from the real connection
// (not append), so the upstream cannot be lied to about the client.
func TestReverseProxyNormalizesForwarded(t *testing.T) {
	var gotXFF, gotForwarded, gotXFHost, gotXFProto string
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotXFF = r.Header.Get("X-Forwarded-For")
		gotForwarded = r.Header.Get("Forwarded")
		gotXFHost = r.Header.Get("X-Forwarded-Host")
		gotXFProto = r.Header.Get("X-Forwarded-Proto")
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	target, err := url.Parse(backend.URL)
	if err != nil {
		t.Fatal(err)
	}
	proxy := newReverseProxy(target)

	front := httptest.NewServer(proxy)
	defer front.Close()

	req, err := http.NewRequest(http.MethodGet, front.URL+"/api/x", nil)
	if err != nil {
		t.Fatal(err)
	}
	// Spoofed inbound headers a malicious client might send.
	req.Header.Set("X-Forwarded-For", "1.2.3.4")
	req.Header.Set("X-Forwarded-Host", "evil.example.com")
	req.Header.Set("X-Forwarded-Proto", "https")
	req.Header.Set("Forwarded", "for=1.2.3.4;host=evil.example.com")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	// Spoofed value must be replaced, not appended; the real client is 127.0.0.1.
	if strings.Contains(gotXFF, "1.2.3.4") {
		t.Errorf("X-Forwarded-For still contains spoofed value: %q", gotXFF)
	}
	if gotXFF == "" {
		t.Error("X-Forwarded-For was not set from the real connection")
	}
	if !strings.HasPrefix(gotXFF, "127.0.0.1") {
		t.Errorf("X-Forwarded-For = %q, want it to start with the real client 127.0.0.1", gotXFF)
	}
	// The client-supplied Forwarded header is dropped (SetXForwarded does not set
	// the RFC 7239 Forwarded header, only the X-Forwarded-* family).
	if strings.Contains(gotForwarded, "evil.example.com") {
		t.Errorf("Forwarded header leaked spoofed host: %q", gotForwarded)
	}
	// SetXForwarded resets X-Forwarded-Host/Proto, so the spoofed values must not survive.
	if gotXFHost == "evil.example.com" {
		t.Errorf("X-Forwarded-Host leaked spoofed value: %q", gotXFHost)
	}
	if gotXFProto == "https" {
		t.Errorf("X-Forwarded-Proto leaked spoofed value: %q", gotXFProto)
	}
}

func TestReverseProxyKeepsFlushInterval(t *testing.T) {
	target, _ := url.Parse("http://example.com")
	proxy := newReverseProxy(target)
	if proxy.FlushInterval != -1 {
		t.Errorf("FlushInterval = %d, want -1 (SSE streaming)", proxy.FlushInterval)
	}
	if proxy.Rewrite == nil {
		t.Error("Rewrite must be set")
	}
	if proxy.Director != nil {
		t.Error("Director must be nil when Rewrite is used")
	}
}
