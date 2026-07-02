// Package recaptcha verifies CAPTCHA response tokens against a provider's
// siteverify endpoint. Google reCAPTCHA v2 and Cloudflare Turnstile share the same
// POST contract (secret+response+remoteip in, {success, error-codes} out), so one
// verifier serves both. An empty secret yields a nil Verifier, so verification is
// off and callers treat it as disabled.
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

	"github.com/Hayao0819/Kamisato/internal/httpx"
)

const (
	recaptchaVerifyURL = "https://www.google.com/recaptcha/api/siteverify"
	turnstileVerifyURL = "https://challenges.cloudflare.com/turnstile/v0/siteverify"
)

// Verifier checks a CAPTCHA response token submitted by a client.
type Verifier interface {
	Verify(ctx context.Context, token, remoteIP string) error
}

// New returns a Verifier for the given provider and secret, or nil when secret is
// empty (verification disabled). Provider "turnstile" targets Cloudflare Turnstile;
// any other value (including "recaptcha" and empty) targets Google reCAPTCHA.
func New(provider, secret string) Verifier {
	if secret == "" {
		return nil
	}
	endpoint := recaptchaVerifyURL
	if provider == "turnstile" {
		endpoint = turnstileVerifyURL
	}
	return &verifier{
		secret:   secret,
		endpoint: endpoint,
		client:   httpx.New(10*time.Second, 3),
	}
}

type verifier struct {
	secret   string
	endpoint string
	client   *http.Client
}

func (v *verifier) Verify(ctx context.Context, token, remoteIP string) error {
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
