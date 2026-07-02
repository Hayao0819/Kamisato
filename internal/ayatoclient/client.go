// Package ayatoclient is a thin HTTP client for the ayato-exposed build/jobs
// API. Clients (lumine, ayaka) normally talk to ayato, which proxies build and
// job requests to the internal miko build server. miko exposes the same
// /api/unstable build/jobs endpoints and accepts the same Bearer token, so
// pointing the base URL straight at miko (thoma's direct mode) reuses these
// calls unchanged.
package ayatoclient

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Hayao0819/Kamisato/internal/utils"
)

// apiClient bounds a regular JSON API call so a hung ayato cannot hang the CLI.
// Streaming and download requests use streamClient instead — a total timeout
// would abort a long log stream or a large package transfer — and rely on
// context cancellation.
var (
	apiClient    = &http.Client{Timeout: 30 * time.Second}
	streamClient = &http.Client{}
)

// endpoint joins the ayato base URL with an API path, tolerating a trailing
// slash on the base.
func endpoint(base, p string) string {
	return strings.TrimRight(base, "/") + p
}

// get issues a GET carrying ctx (for cancellation and, on apiClient, the
// timeout).
func get(ctx context.Context, client *http.Client, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
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
		return utils.NewErrf("%s failed: %s", op, resp.Status)
	}
	return utils.NewErrf("%s failed: %s: %s", op, resp.Status, msg)
}
