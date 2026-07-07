package bugreport

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Hayao0819/Kamisato/internal/httpx"
)

const webhookEvent = "bug_report"

// WebhookPayload is the stable wire format POSTed to a webhook. It is a separate
// DTO from Report so MaintainerEmail is never forwarded and the snake_case names
// stay stable for consumers as Report evolves.
type WebhookPayload struct {
	Event     string      `json:"event"`
	ID        string      `json:"id"`
	Timestamp time.Time   `json:"timestamp"`
	Data      WebhookData `json:"data"`
}

// WebhookData is the reporter-supplied subset of a Report. MaintainerEmail is
// intentionally absent.
type WebhookData struct {
	Pkgname     string `json:"pkgname"`
	Pkgver      string `json:"pkgver"`
	Name        string `json:"name"`
	Email       string `json:"email"`
	Severity    string `json:"severity"`
	Description string `json:"description"`
}

// webhookReporter POSTs the report as JSON to an arbitrary operator-supplied endpoint.
type webhookReporter struct {
	client *http.Client
	url    string
}

func newWebhook(cfg WebhookConfig) (Reporter, error) {
	if cfg.URL == "" {
		return nil, fmt.Errorf("bugreport: webhook url is required")
	}
	return &webhookReporter{client: httpx.New(10*time.Second, 3), url: cfg.URL}, nil
}

func (h *webhookReporter) Report(ctx context.Context, r Report) (string, error) {
	buf, err := json.Marshal(toWebhookPayload(r))
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, h.url, bytes.NewReader(buf))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := h.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("bugreport: webhook request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<10))
		return "", fmt.Errorf("bugreport: webhook returned %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	return "", nil
}

func toWebhookPayload(r Report) WebhookPayload {
	return WebhookPayload{
		Event:     webhookEvent,
		ID:        newDeliveryID(),
		Timestamp: time.Now().UTC(),
		Data: WebhookData{
			Pkgname:     r.Pkgname,
			Pkgver:      r.Pkgver,
			Name:        r.Name,
			Email:       r.Email,
			Severity:    r.Severity,
			Description: r.Description,
		},
	}
}

// newDeliveryID gives each delivery a unique id for correlation/de-dup.
// crypto/rand.Read never fails on our target platforms, so the error is ignored.
func newDeliveryID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}
