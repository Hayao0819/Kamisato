package ayato

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strconv"

	"github.com/Hayao0819/Kamisato/internal/api/client"
)

// CLIStartURL returns the CLI login URL.
func (c *Client) CLIStartURL(port int, challenge, state string) string {
	target := c.request.Transport().Endpoint("api", "unstable", "auth", "cli", "start")
	query := target.Query()
	query.Set("port", strconv.Itoa(port))
	query.Set("challenge", challenge)
	query.Set("state", state)
	target.RawQuery = query.Encode()
	return target.String()
}

func (c *Client) ExchangeCLICode(ctx context.Context, code, verifier string) (TokenPair, error) {
	payload := struct {
		Code         string `json:"code"`
		CodeVerifier string `json:"code_verifier"`
	}{Code: code, CodeVerifier: verifier}
	var result TokenPair
	err := c.request.Transport().DoJSON(
		ctx,
		client.NoRetry,
		http.MethodPost,
		c.request.Transport().Endpoint("api", "unstable", "auth", "cli", "exchange"),
		false,
		payload,
		&result,
		http.StatusOK,
		"cli exchange",
	)
	return result, err
}

// RefreshAccessToken rotates an access and refresh token pair.
func (c *Client) RefreshAccessToken(ctx context.Context, refreshToken string) (TokenPair, error) {
	payload := struct {
		RefreshToken string `json:"refresh_token"`
	}{RefreshToken: refreshToken}
	var result TokenPair
	err := c.request.Transport().DoJSON(
		ctx,
		client.NoRetry,
		http.MethodPost,
		c.request.Transport().Endpoint("api", "unstable", "auth", "refresh"),
		false,
		payload,
		&result,
		http.StatusOK,
		"token refresh",
	)
	return result, err
}

func (c *Client) ListAdmins(ctx context.Context) ([]Admin, error) {
	var result struct {
		Admins []Admin `json:"admins"`
	}
	err := c.request.Execute(ctx, func() error {
		return c.request.Transport().DoJSON(
			ctx,
			client.RetryReplaySafe,
			http.MethodGet,
			c.request.Transport().Endpoint("api", "unstable", "auth", "admins"),
			true,
			nil,
			&result,
			http.StatusOK,
			"list admins",
		)
	})
	return result.Admins, err
}

func (c *Client) AddAdmin(ctx context.Context, id int64, login string) (Admin, error) {
	var payload struct {
		ID    int64  `json:"id,omitempty"`
		Login string `json:"login,omitempty"`
	}
	if id == 0 {
		payload.Login = login
	} else {
		payload.ID = id
	}
	var result Admin
	err := c.request.Execute(ctx, func() error {
		return c.request.Transport().DoJSON(
			ctx,
			client.NoRetry,
			http.MethodPost,
			c.request.Transport().Endpoint("api", "unstable", "auth", "admins"),
			true,
			payload,
			&result,
			http.StatusOK,
			"add admin",
		)
	})
	return result, err
}

func (c *Client) RemoveAdmin(ctx context.Context, id int64) error {
	return c.request.Execute(ctx, func() error {
		return c.request.Transport().DoJSON(
			ctx,
			client.NoRetry,
			http.MethodDelete,
			c.request.Transport().Endpoint("api", "unstable", "auth", "admins", strconv.FormatInt(id, 10)),
			true,
			nil,
			nil,
			http.StatusOK,
			"remove admin",
		)
	})
}

func (c *Client) RevokeCLIToken(ctx context.Context, refreshToken string) error {
	var payload any
	if refreshToken != "" {
		payload = struct {
			RefreshToken string `json:"refresh_token"`
		}{RefreshToken: refreshToken}
	}
	return c.request.Execute(ctx, func() error {
		return c.request.Transport().DoJSON(
			ctx,
			client.NoRetry,
			http.MethodPost,
			c.request.Transport().Endpoint("api", "unstable", "auth", "cli", "revoke"),
			true,
			payload,
			nil,
			http.StatusOK,
			"revoke token",
		)
	})
}

func (c *Client) RequestDeviceCode(ctx context.Context) (DeviceCodeResponse, error) {
	var result DeviceCodeResponse
	err := c.request.Transport().DoJSON(
		ctx,
		client.NoRetry,
		http.MethodPost,
		c.request.Transport().Endpoint("api", "unstable", "auth", "device", "code"),
		false,
		nil,
		&result,
		http.StatusOK,
		"device code",
	)
	return result, err
}

func (c *Client) PollDeviceToken(ctx context.Context, deviceCode string) (DeviceTokenResult, error) {
	payload := struct {
		DeviceCode string `json:"device_code"`
	}{DeviceCode: deviceCode}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return DeviceTokenResult{}, err
	}
	req, err := c.request.Transport().NewRequest(
		ctx,
		http.MethodPost,
		c.request.Transport().Endpoint("api", "unstable", "auth", "device", "token"),
		bytes.NewReader(encoded),
		false,
	)
	if err != nil {
		return DeviceTokenResult{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.request.Transport().Do(req)
	if err != nil {
		return DeviceTokenResult{}, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	var envelope struct {
		Token        string `json:"token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
		Login        string `json:"login"`
		ID           int64  `json:"id"`
		Error        string `json:"error"`
	}
	_ = json.Unmarshal(body, &envelope)
	switch {
	case resp.StatusCode == http.StatusOK && envelope.Token != "":
		return DeviceTokenResult{
			Token:     envelope.Token,
			Refresh:   envelope.RefreshToken,
			ExpiresIn: envelope.ExpiresIn,
			Login:     envelope.Login,
			ID:        envelope.ID,
		}, nil
	case resp.StatusCode == http.StatusBadRequest && envelope.Error != "":
		return DeviceTokenResult{Status: envelope.Error}, nil
	default:
		return DeviceTokenResult{}, &ResponseError{StatusCode: resp.StatusCode, Message: envelope.Error}
	}
}
