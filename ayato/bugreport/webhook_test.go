package bugreport

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWebhookReport(t *testing.T) {
	var gotMethod, gotType string
	var got Report
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotType = r.Header.Get("Content-Type")
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &got)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	h := &webhookReporter{client: srv.Client(), url: srv.URL}
	url, err := h.Report(context.Background(), Report{Pkgname: "foo", Severity: "high", Description: "boom"})
	if err != nil {
		t.Fatalf("Report: %v", err)
	}
	if url != "" {
		t.Errorf("webhook url = %q, want empty", url)
	}
	if gotMethod != http.MethodPost {
		t.Errorf("method = %q, want POST", gotMethod)
	}
	if gotType != "application/json" {
		t.Errorf("content-type = %q, want application/json", gotType)
	}
	if got.Pkgname != "foo" || got.Severity != "high" || got.Description != "boom" {
		t.Errorf("posted body = %+v", got)
	}
}

func TestWebhookReportNon2xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	h := &webhookReporter{client: srv.Client(), url: srv.URL}
	if _, err := h.Report(context.Background(), Report{Pkgname: "foo", Description: "x"}); err == nil {
		t.Error("a non-2xx response must surface an error")
	}
}

func TestNewWebhookRequiresURL(t *testing.T) {
	if _, err := newWebhook(WebhookConfig{}); err == nil {
		t.Error("webhook without a url must error")
	}
}
