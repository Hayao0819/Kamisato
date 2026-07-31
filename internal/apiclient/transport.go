package apiclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Hayao0819/Kamisato/internal/errors"
)

const accessTokenExpiredHeader = "X-Access-Token-Expired" //nolint:gosec // header name, not a credential

var ErrAccessTokenExpired = errors.New("access token expired")

// ResponseError represents a non-success HTTP response.
type ResponseError struct {
	StatusCode int
	Message    string
	Reason     string
}

func (e *ResponseError) Error() string {
	if e.Reason != "" && e.Reason != e.Message {
		return fmt.Sprintf("request failed: status %d: %s: %s", e.StatusCode, e.Message, e.Reason)
	}
	if e.Message != "" {
		return fmt.Sprintf("request failed: status %d: %s", e.StatusCode, e.Message)
	}
	return fmt.Sprintf("request failed: status %d", e.StatusCode)
}

// BearerTokenSource supplies an ayato user token at request time.
type BearerTokenSource interface {
	Token(context.Context) (string, error)
}

// RefreshableBearerTokenSource can rotate an expired token.
type RefreshableBearerTokenSource interface {
	BearerTokenSource
	RefreshIfCurrent(context.Context, string) error
}

type BearerTokenSourceFunc func(context.Context) (string, error)

func (f BearerTokenSourceFunc) Token(ctx context.Context) (string, error) { return f(ctx) }

func StaticBearer(token string) BearerTokenSource {
	return BearerTokenSourceFunc(func(context.Context) (string, error) { return token, nil })
}

type credential interface {
	apply(context.Context, *http.Request) error
}

type noCredential struct{}

func (noCredential) apply(context.Context, *http.Request) error { return nil }

type bearerCredential struct{ source BearerTokenSource }

func (c bearerCredential) apply(ctx context.Context, req *http.Request) error {
	if c.source == nil {
		return nil
	}
	token, err := c.source.Token(ctx)
	if err != nil {
		return err
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	return nil
}

type apiKeyCredential struct{ key string }

func (c apiKeyCredential) apply(_ context.Context, req *http.Request) error {
	if c.key != "" {
		req.Header.Set("X-API-Key", c.key)
	}
	return nil
}

type options struct {
	httpClient     *http.Client
	attemptTimeout time.Duration
	readAttempts   int
}

type Option func(*options)

func WithHTTPClient(c *http.Client) Option {
	return func(o *options) { o.httpClient = c }
}

func WithReadAttempts(attempts int) Option {
	return func(o *options) { o.readAttempts = attempts }
}

type RetryPolicy uint8

const (
	NoRetry RetryPolicy = iota
	RetryReplaySafe
)

type Transport struct {
	base           *url.URL
	http           *http.Client
	credential     credential
	attemptTimeout time.Duration
	readAttempts   int
}

func newTransport(rawBase string, auth credential, opts ...Option) (*Transport, error) {
	base, err := ParseBaseURL(rawBase)
	if err != nil {
		return nil, err
	}
	cfg := options{attemptTimeout: 30 * time.Second, readAttempts: 3}
	for _, opt := range opts {
		opt(&cfg)
	}
	if cfg.httpClient == nil {
		cfg.httpClient = &http.Client{}
	}
	if cfg.readAttempts < 1 {
		cfg.readAttempts = 1
	}
	return &Transport{
		base:           base,
		http:           secureRedirectClient(cfg.httpClient),
		credential:     auth,
		attemptTimeout: cfg.attemptTimeout,
		readAttempts:   cfg.readAttempts,
	}, nil
}

func NewPublicTransport(rawBase string, opts ...Option) (*Transport, error) {
	return newTransport(rawBase, noCredential{}, opts...)
}

func newBearerTransport(rawBase string, source BearerTokenSource, opts ...Option) (*Transport, error) {
	return newTransport(rawBase, bearerCredential{source: source}, opts...)
}

func newAPIKeyTransport(rawBase, key string, opts ...Option) (*Transport, error) {
	return newTransport(rawBase, apiKeyCredential{key: key}, opts...)
}

// ParseBaseURL validates a component endpoint.
func ParseBaseURL(raw string) (*url.URL, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return nil, errors.WrapErr(err, "parse server URL")
	}
	if (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, errors.NewErr("server URL must be an absolute http or https URL")
	}
	if u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return nil, errors.NewErr("server URL must not contain userinfo, query, or fragment")
	}
	u.Path = strings.TrimRight(u.Path, "/")
	u.RawPath = strings.TrimRight(u.RawPath, "/")
	return u, nil
}

func secureUserEndpoint(u *url.URL) bool {
	if strings.EqualFold(u.Scheme, "https") {
		return true
	}
	host := u.Hostname()
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func (t *Transport) SecureUserEndpoint() bool {
	return secureUserEndpoint(t.base)
}

func (t *Transport) Endpoint(segments ...string) *url.URL {
	return EndpointURL(t.base, segments...)
}

func (t *Transport) AttemptContext(ctx context.Context) (context.Context, context.CancelFunc) {
	if t.attemptTimeout <= 0 {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, t.attemptTimeout)
}

// EndpointURL appends escaped path segments to a base URL.
func EndpointURL(base *url.URL, segments ...string) *url.URL {
	u := *base
	rawPath := strings.TrimRight(u.EscapedPath(), "/")
	for _, segment := range segments {
		rawPath += "/" + url.PathEscape(segment)
	}
	path, err := url.PathUnescape(rawPath)
	if err != nil {
		path = rawPath
	}
	u.Path = path
	u.RawPath = rawPath
	return &u
}

func secureRedirectClient(base *http.Client) *http.Client {
	c := *base
	previous := c.CheckRedirect
	c.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		stripCredentials(req.Header)
		if len(via) > 0 && hasCredential(via[0].Header) {
			return http.ErrUseLastResponse
		}
		if previous != nil {
			return previous(req, via)
		}
		if len(via) >= 10 {
			return errors.NewErr("stopped after 10 redirects")
		}
		return nil
	}
	return &c
}

func hasCredential(h http.Header) bool {
	return h.Get("Authorization") != "" || h.Get("Cookie") != "" || h.Get("X-API-Key") != "" || h.Get("X-Log-Token") != ""
}

func stripCredentials(h http.Header) {
	for _, name := range []string{"Authorization", "Cookie", "X-API-Key", "X-Log-Token"} {
		h.Del(name)
	}
}

func (t *Transport) NewRequest(ctx context.Context, method string, target *url.URL, body io.Reader, authenticated bool) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, target.String(), body)
	if err != nil {
		return nil, err
	}
	if authenticated {
		if !sameOrigin(target, t.base) {
			return nil, errors.NewErr("refusing to send credentials to a different origin")
		}
		if err := t.credential.apply(ctx, req); err != nil {
			return nil, errors.WrapErr(err, "resolve request credential")
		}
	}
	return req, nil
}

func sameOrigin(a, b *url.URL) bool {
	return strings.EqualFold(a.Scheme, b.Scheme) && strings.EqualFold(a.Hostname(), b.Hostname()) && effectivePort(a) == effectivePort(b)
}

func effectivePort(u *url.URL) string {
	if p := u.Port(); p != "" {
		return p
	}
	if strings.EqualFold(u.Scheme, "https") {
		return "443"
	}
	return "80"
}

func (t *Transport) DoJSON(ctx context.Context, policy RetryPolicy, method string, target *url.URL, authenticated bool, body, out any, want int, op string) error {
	var encoded []byte
	var err error
	if body != nil {
		encoded, err = json.Marshal(body)
		if err != nil {
			return errors.WrapErr(err, "encode "+op+" request")
		}
	}
	attempts := 1
	if policy == RetryReplaySafe {
		attempts = t.readAttempts
	}
	for attempt := 0; attempt < attempts; attempt++ {
		attemptCtx, cancel := t.AttemptContext(ctx)
		var reader io.Reader
		if body != nil {
			reader = bytes.NewReader(encoded)
		}
		req, reqErr := t.NewRequest(attemptCtx, method, target, reader, authenticated)
		if reqErr != nil {
			cancel()
			return errors.WrapErr(reqErr, "create "+op+" request")
		}
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		resp, sendErr := t.http.Do(req)
		if sendErr != nil {
			cancel()
			if attempt+1 < attempts && ctx.Err() == nil {
				if err := retryDelay(ctx, attempt); err != nil {
					return err
				}
				continue
			}
			return errors.WrapErr(sendErr, "send "+op+" request")
		}

		if resp.StatusCode != want {
			if attempt+1 < attempts && retryableStatus(resp.StatusCode) {
				_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
				_ = resp.Body.Close()
				cancel()
				if err := retryDelay(ctx, attempt); err != nil {
					return err
				}
				continue
			}
			err := ReadResponseError(resp, op)
			cancel()
			return err
		}
		if out != nil {
			if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
				_ = resp.Body.Close()
				cancel()
				return errors.WrapErr(err, "decode "+op+" response")
			}
		}
		_ = resp.Body.Close()
		cancel()
		return nil
	}
	return errors.NewErr("unreachable request retry state")
}

func retryableStatus(status int) bool {
	return status == http.StatusTooManyRequests || status == 0 || (status >= 500 && status != http.StatusNotImplemented)
}

func retryDelay(ctx context.Context, attempt int) error {
	delay := 100 * time.Millisecond * time.Duration(1<<min(attempt, 3))
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func ReadResponseError(resp *http.Response, op string) error {
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized && resp.Header.Get(accessTokenExpiredHeader) == "1" {
		return errors.WrapErr(ErrAccessTokenExpired, op)
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	var envelope struct {
		Error   string `json:"error"`
		Message string `json:"message"`
		Reason  string `json:"reason"`
	}
	_ = json.Unmarshal(body, &envelope)
	message := strings.TrimSpace(envelope.Error)
	if message == "" {
		message = strings.TrimSpace(envelope.Message)
	}
	if message == "" {
		message = strings.TrimSpace(string(body))
	}
	if message == "" {
		message = http.StatusText(resp.StatusCode)
	}
	return errors.WrapErr(&ResponseError{StatusCode: resp.StatusCode, Message: message, Reason: strings.TrimSpace(envelope.Reason)}, op)
}

func (t *Transport) Get(ctx context.Context, target *url.URL, authenticated bool) (*http.Response, error) {
	req, err := t.NewRequest(ctx, http.MethodGet, target, nil, authenticated)
	if err != nil {
		return nil, err
	}
	return t.http.Do(req)
}

func (t *Transport) Do(request *http.Request) (*http.Response, error) {
	return t.http.Do(request) //nolint:gosec // Callers build requests from the transport's validated base URL.
}

func (t *Transport) ReadBytes(
	ctx context.Context,
	targetURL *url.URL,
	maxBytes int64,
	operation string,
	accept string,
) ([]byte, error) {
	for attempt := 0; attempt < t.readAttempts; attempt++ {
		attemptCtx, cancel := t.AttemptContext(ctx)
		request, err := t.NewRequest(attemptCtx, http.MethodGet, targetURL, nil, false)
		if err != nil {
			cancel()
			return nil, errors.WrapErr(err, "create "+operation+" request")
		}
		if accept != "" {
			request.Header.Set("Accept", accept)
		}
		response, err := t.http.Do(request)
		if err != nil {
			cancel()
			if attempt+1 < t.readAttempts && ctx.Err() == nil {
				if err := retryDelay(ctx, attempt); err != nil {
					return nil, err
				}
				continue
			}
			return nil, errors.WrapErr(err, operation)
		}
		if response.StatusCode != http.StatusOK {
			if attempt+1 < t.readAttempts && retryableStatus(response.StatusCode) {
				_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
				_ = response.Body.Close()
				cancel()
				if err := retryDelay(ctx, attempt); err != nil {
					return nil, err
				}
				continue
			}
			err := ReadResponseError(response, operation)
			cancel()
			return nil, err
		}
		if response.ContentLength > maxBytes {
			_ = response.Body.Close()
			cancel()
			return nil, errors.NewErrf("%s response exceeds %d bytes", operation, maxBytes)
		}
		body, readErr := io.ReadAll(io.LimitReader(response.Body, maxBytes+1))
		_ = response.Body.Close()
		cancel()
		if readErr != nil {
			return nil, errors.WrapErr(readErr, operation)
		}
		if int64(len(body)) > maxBytes {
			return nil, errors.NewErrf("%s response exceeds %d bytes", operation, maxBytes)
		}
		return body, nil
	}
	return nil, errors.NewErr("unreachable public read retry state")
}
