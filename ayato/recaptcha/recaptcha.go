// Package recaptcha verifies Google reCAPTCHA v2 tokens. An empty secret yields a
// nil Verifier, so verification is simply off and callers treat it as disabled.
package recaptcha

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const verifyURL = "https://www.google.com/recaptcha/api/siteverify"

// Verifier checks a reCAPTCHA response token submitted by a client.
type Verifier interface {
	Verify(ctx context.Context, token, remoteIP string) error
}

// New returns a Verifier for the given secret, or nil when secret is empty
// (reCAPTCHA disabled).
func New(secret string) Verifier {
	if secret == "" {
		return nil
	}
	return &googleVerifier{
		secret:   secret,
		endpoint: verifyURL,
		client:   &http.Client{Timeout: 10 * time.Second},
	}
}

type googleVerifier struct {
	secret   string
	endpoint string
	client   *http.Client
}

func (v *googleVerifier) Verify(ctx context.Context, token, remoteIP string) error {
	if token == "" {
		return fmt.Errorf("recaptcha: missing response token")
	}
	form := url.Values{"secret": {v.secret}, "response": {token}}
	if remoteIP != "" {
		form.Set("remoteip", remoteIP)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, v.endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := v.client.Do(req)
	if err != nil {
		return fmt.Errorf("recaptcha: verify request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<10))
		return fmt.Errorf("recaptcha: verify endpoint returned %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	var out struct {
		Success    bool     `json:"success"`
		ErrorCodes []string `json:"error-codes"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return err
	}
	if !out.Success {
		return fmt.Errorf("recaptcha: verification failed: %v", out.ErrorCodes)
	}
	return nil
}
