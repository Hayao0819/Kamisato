// Package platform contains Ayato's layer-neutral technical building blocks:
// HTTP representations and security, process lifecycle, shared rate limits, and
// portable file streams.
//
// It does not depend on Ayato's domain, service, repository, or transport layers.
// Higher layers can therefore share its capabilities without reversing their
// dependency direction.
package platform

import (
	"net/http"
	"net/url"
	"strings"
)

type HTTPErrorCode string

const (
	HTTPCodeRequestFailed      HTTPErrorCode = "request_failed"
	HTTPCodeInvalidRequest     HTTPErrorCode = "invalid_request"
	HTTPCodeUnauthorized       HTTPErrorCode = "unauthorized"
	HTTPCodeForbidden          HTTPErrorCode = "forbidden"
	HTTPCodeNotFound           HTTPErrorCode = "not_found"
	HTTPCodeConflict           HTTPErrorCode = "conflict"
	HTTPCodePayloadTooLarge    HTTPErrorCode = "payload_too_large"
	HTTPCodeRateLimited        HTTPErrorCode = "rate_limited"
	HTTPCodeInternal           HTTPErrorCode = "internal_error"
	HTTPCodeNotImplemented     HTTPErrorCode = "not_implemented"
	HTTPCodeUpstream           HTTPErrorCode = "upstream_error"
	HTTPCodeServiceUnavailable HTTPErrorCode = "service_unavailable"
)

// HTTPErrorResponse is the public error envelope for non-protocol Ayato APIs.
// Message is safe for clients; implementation causes belong in server logs.
type HTTPErrorResponse struct {
	Code    HTTPErrorCode `json:"code"`
	Message string        `json:"message"`
}

func NewHTTPError(status int, message string) HTTPErrorResponse {
	return HTTPErrorResponse{Code: HTTPErrorCodeForStatus(status), Message: message}
}

func HTTPErrorCodeForStatus(status int) HTTPErrorCode {
	switch status {
	case http.StatusBadRequest, http.StatusUnprocessableEntity:
		return HTTPCodeInvalidRequest
	case http.StatusUnauthorized:
		return HTTPCodeUnauthorized
	case http.StatusForbidden:
		return HTTPCodeForbidden
	case http.StatusNotFound:
		return HTTPCodeNotFound
	case http.StatusConflict:
		return HTTPCodeConflict
	case http.StatusRequestEntityTooLarge:
		return HTTPCodePayloadTooLarge
	case http.StatusTooManyRequests:
		return HTTPCodeRateLimited
	case http.StatusNotImplemented:
		return HTTPCodeNotImplemented
	case http.StatusBadGateway:
		return HTTPCodeUpstream
	case http.StatusServiceUnavailable:
		return HTTPCodeServiceUnavailable
	default:
		if status >= http.StatusInternalServerError {
			return HTTPCodeInternal
		}
		return HTTPCodeRequestFailed
	}
}

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
