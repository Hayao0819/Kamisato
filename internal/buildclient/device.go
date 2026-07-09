package buildclient

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"

	"github.com/Hayao0819/Kamisato/internal/errors"
)

// DeviceCodeResponse is ayato's reply to a device authorization request (RFC 8628
// §3.2): the codes to show the user plus the polling parameters.
type DeviceCodeResponse struct {
	DeviceCode              string `json:"device_code"`
	UserCode                string `json:"user_code"`
	VerificationURI         string `json:"verification_uri"`
	VerificationURIComplete string `json:"verification_uri_complete"`
	ExpiresIn               int    `json:"expires_in"`
	Interval                int    `json:"interval"`
}

// DeviceTokenResult is one polling outcome. On approval Token (plus Refresh, Login,
// ID) is set and Status is empty; otherwise Status carries the RFC 8628 error the
// caller acts on (authorization_pending, slow_down, access_denied, expired_token).
type DeviceTokenResult struct {
	Token   string
	Refresh string
	Login   string
	ID      int64
	Status  string
}

// RequestDeviceCode starts a device authorization, returning the codes to display
// and the polling parameters. It needs no credentials.
func RequestDeviceCode(ctx context.Context, base string) (DeviceCodeResponse, error) {
	var out DeviceCodeResponse
	if err := doJSON(ctx, http.MethodPost, endpoint(base, "/api/unstable/auth/device/code"), "", nil, &out, http.StatusOK, "device code"); err != nil {
		return DeviceCodeResponse{}, err
	}
	return out, nil
}

// PollDeviceToken polls once for the token. A pending/slow_down/denied/expired
// authorization comes back as a Status (not a Go error) so the caller can keep
// polling or stop; only a transport or unexpected-status failure is an error.
func PollDeviceToken(ctx context.Context, base, deviceCode string) (DeviceTokenResult, error) {
	reqBody := struct {
		DeviceCode string `json:"device_code"`
	}{DeviceCode: deviceCode}
	encoded, err := json.Marshal(reqBody)
	if err != nil {
		return DeviceTokenResult{}, errors.WrapErr(err, "failed to encode device token request")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint(base, "/api/unstable/auth/device/token"), bytes.NewReader(encoded))
	if err != nil {
		return DeviceTokenResult{}, errors.WrapErr(err, "failed to create device token request")
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := apiClient.Do(req)
	if err != nil {
		return DeviceTokenResult{}, errors.WrapErr(err, "failed to send device token request")
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	var out struct {
		Token        string `json:"token"`
		RefreshToken string `json:"refresh_token"`
		Login        string `json:"login"`
		ID           int64  `json:"id"`
		Error        string `json:"error"`
	}
	_ = json.Unmarshal(body, &out)

	switch {
	case resp.StatusCode == http.StatusOK && out.Token != "":
		return DeviceTokenResult{Token: out.Token, Refresh: out.RefreshToken, Login: out.Login, ID: out.ID}, nil
	case resp.StatusCode == http.StatusBadRequest && out.Error != "":
		return DeviceTokenResult{Status: out.Error}, nil
	default:
		return DeviceTokenResult{}, errors.NewErrf("device token poll failed: %s: %s", resp.Status, out.Error)
	}
}
