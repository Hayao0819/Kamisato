// Package httpsecurity contains transport-level checks shared by Ayato handlers
// and middleware.
package httpsecurity

import (
	"net/http"
	"net/url"
	"strings"
)

// SameOrigin verifies the browser's request-origin signal. Sec-Fetch-Site is
// authoritative when present. Older clients fall back to Origin, then Referer,
// matched against normalized allowed origins. Missing or malformed signals fail
// closed.
func SameOrigin(request *http.Request, allowedOrigins ...string) bool {
	if request == nil {
		return false
	}
	if fetchSite := request.Header.Get("Sec-Fetch-Site"); fetchSite != "" {
		return fetchSite == "same-origin"
	}

	presented := request.Header.Get("Origin")
	if presented == "" {
		presented = request.Header.Get("Referer")
	}
	presented = Origin(presented)
	if presented == "" {
		return false
	}
	for _, allowed := range allowedOrigins {
		if normalized := Origin(allowed); normalized != "" &&
			strings.EqualFold(presented, normalized) {
			return true
		}
	}
	return false
}

// Origin normalizes an absolute URL to scheme://host, or returns an empty string
// for an invalid or relative URL.
func Origin(raw string) string {
	_, origin, ok := ParseOrigin(raw)
	if !ok {
		return ""
	}
	return origin
}

// ParseOrigin normalizes an absolute URL and also returns its scheme.
func ParseOrigin(raw string) (scheme, origin string, ok bool) {
	if raw == "" {
		return "", "", false
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", "", false
	}
	return parsed.Scheme, parsed.Scheme + "://" + parsed.Host, true
}
