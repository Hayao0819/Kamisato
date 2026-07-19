package httpsecurity

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSameOrigin(t *testing.T) {
	tests := []struct {
		name    string
		headers map[string]string
		allowed []string
		want    bool
	}{
		{
			name:    "fetch metadata accepts same origin",
			headers: map[string]string{"Sec-Fetch-Site": "same-origin"},
			want:    true,
		},
		{
			name:    "fetch metadata rejects cross site",
			headers: map[string]string{"Sec-Fetch-Site": "cross-site", "Origin": "https://ayato.example"},
			allowed: []string{"https://ayato.example"},
		},
		{
			name:    "origin matches normalized allowlist",
			headers: map[string]string{"Origin": "https://ayato.example"},
			allowed: []string{"https://ayato.example/path"},
			want:    true,
		},
		{
			name:    "origin scheme must match",
			headers: map[string]string{"Origin": "http://ayato.example"},
			allowed: []string{"https://ayato.example"},
		},
		{
			name:    "referer fallback",
			headers: map[string]string{"Referer": "https://ayato.example/settings"},
			allowed: []string{"https://ayato.example"},
			want:    true,
		},
		{
			name:    "missing signal fails closed",
			allowed: []string{"https://ayato.example"},
		},
		{
			name:    "malformed signal fails closed",
			headers: map[string]string{"Origin": "://bad"},
			allowed: []string{"https://ayato.example"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "https://ayato.example/logout", nil)
			for name, value := range test.headers {
				request.Header.Set(name, value)
			}
			if got := SameOrigin(request, test.allowed...); got != test.want {
				t.Errorf("SameOrigin() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestOrigin(t *testing.T) {
	tests := []struct {
		raw, scheme, origin string
		ok                  bool
	}{
		{raw: "https://example.com/path?q=1", scheme: "https", origin: "https://example.com", ok: true},
		{raw: "http://example.com:8080/", scheme: "http", origin: "http://example.com:8080", ok: true},
		{raw: "/relative"},
		{raw: "://bad"},
		{},
	}
	for _, test := range tests {
		if got := Origin(test.raw); got != test.origin {
			t.Errorf("Origin(%q) = %q, want %q", test.raw, got, test.origin)
		}
		scheme, origin, ok := ParseOrigin(test.raw)
		if scheme != test.scheme || origin != test.origin || ok != test.ok {
			t.Errorf(
				"ParseOrigin(%q) = (%q, %q, %v), want (%q, %q, %v)",
				test.raw, scheme, origin, ok, test.scheme, test.origin, test.ok,
			)
		}
	}
}
