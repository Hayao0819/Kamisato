// Package httpx builds the outbound HTTP clients used to reach third-party
// services (AUR, GitHub, reCAPTCHA, package mirrors). Those calls cross a
// network we do not control, so every one shares the same policy here: a
// per-attempt timeout plus a bounded number of retries with exponential
// backoff, so a transient blip or a 5xx does not surface on the first try.
//
// New returns a stdlib *http.Client (via retryablehttp's StandardClient shim)
// rather than a *retryablehttp.Client so call sites keep depending only on
// net/http and can be rewired to this constructor without other changes.
package httpx

import (
	"net/http"
	"time"

	"github.com/hashicorp/go-retryablehttp"
)

const (
	defaultTimeout = 30 * time.Second
	defaultRetries = 3
)

// New returns an *http.Client that bounds each attempt with timeout and retries
// transient failures (connection errors, 5xx) up to retries times using
// exponential backoff.
func New(timeout time.Duration, retries int) *http.Client {
	rc := retryablehttp.NewClient()
	rc.HTTPClient.Timeout = timeout
	rc.RetryMax = retries
	rc.Logger = nil // callers own logging; suppress the per-retry stderr default
	return rc.StandardClient()
}

// Default returns a client tuned for routine external JSON calls.
func Default() *http.Client {
	return New(defaultTimeout, defaultRetries)
}
