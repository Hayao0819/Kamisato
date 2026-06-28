// Package ayatoclient is a thin HTTP client for the ayato-exposed build/jobs
// API. Clients (lumine, ayaka) talk only to ayato, which proxies build and job
// requests to the internal miko build server; this package never contacts miko
// directly.
package ayatoclient

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/Hayao0819/Kamisato/internal/utils"
)

// endpoint joins the ayato base URL with an API path, tolerating a trailing
// slash on the base.
func endpoint(base, p string) string {
	return strings.TrimRight(base, "/") + p
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
