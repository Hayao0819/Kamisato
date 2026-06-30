package bugreport

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// webhookReporter POSTs the report as JSON to an arbitrary endpoint, letting an
// operator wire reports into whatever they already run (chat, a queue, a script).
type webhookReporter struct {
	client *http.Client
	url    string
}

func newWebhook(cfg WebhookConfig) (Reporter, error) {
	if cfg.URL == "" {
		return nil, fmt.Errorf("bugreport: webhook url is required")
	}
	return &webhookReporter{client: &http.Client{Timeout: 10 * time.Second}, url: cfg.URL}, nil
}

func (h *webhookReporter) Report(ctx context.Context, r Report) (string, error) {
	buf, err := json.Marshal(r)
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
