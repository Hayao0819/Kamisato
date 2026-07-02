// Package ayatoclient is a thin HTTP client for the ayato-exposed build/jobs
// API. Clients (lumine, ayaka) normally talk to ayato, which proxies build and
// job requests to the internal miko build server. miko exposes the same
// /api/unstable build/jobs endpoints and accepts the same Bearer token, so
// pointing the base URL straight at miko (thoma's direct mode) reuses these
// calls unchanged.
package ayatoclient

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/Hayao0819/Kamisato/internal/errwrap"
	"github.com/Hayao0819/Kamisato/internal/httpx"
)

// apiClient handles regular JSON API calls; httpx.Default gives it a per-attempt
// timeout and bounded retries so a hung or flaky ayato cannot hang the CLI.
// Streaming and download requests use streamClient instead — a total timeout
// would abort a long log stream or a large package transfer — and rely on
// context cancellation.
var (
	apiClient    = httpx.Default()
	streamClient = &http.Client{}
)

// endpoint joins the ayato base URL with an API path, tolerating a trailing
// slash on the base.
func endpoint(base, p string) string {
	return strings.TrimRight(base, "/") + p
}

// get issues a GET carrying ctx (for cancellation and, on apiClient, the
// timeout), attaching the Bearer token when non-empty.
func get(ctx context.Context, client *http.Client, url, token string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	return client.Do(req)
}

// responseErr builds an error from a non-success response, including any error
// message the server returned in its JSON body.
func responseErr(resp *http.Response, op string) error {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	var apiErr struct {
		Error   string `json:"error"`
		Message string `json:"message"`
	}
	msg := strings.TrimSpace(string(body))
	if json.Unmarshal(body, &apiErr) == nil {
		if apiErr.Error != "" {
			msg = apiErr.Error
		} else if apiErr.Message != "" {
			msg = apiErr.Message
		}
	}
	if msg == "" {
		return errwrap.NewErrf("%s failed: %s", op, resp.Status)
	}
	return errwrap.NewErrf("%s failed: %s: %s", op, resp.Status, msg)
}

// doJSON runs a JSON API call on apiClient: it attaches the Bearer token when
// token is set, sends body as a JSON request body when body is non-nil, maps a
// status other than want to responseErr, and decodes the reply into out when out
// is non-nil. op labels the operation in wrapped errors.
func doJSON(ctx context.Context, method, url, token string, body, out any, want int, op string) error {
	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return errwrap.WrapErr(err, "failed to encode "+op+" request")
		}
		reader = bytes.NewReader(encoded)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, reader)
	if err != nil {
		return errwrap.WrapErr(err, "failed to create "+op+" request")
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := apiClient.Do(req)
	if err != nil {
		return errwrap.WrapErr(err, "failed to send "+op+" request")
	}
	defer resp.Body.Close()

	if resp.StatusCode != want {
		return responseErr(resp, op)
	}
	if out != nil {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			return errwrap.WrapErr(err, "failed to decode "+op+" response")
		}
	}
	return nil
}
