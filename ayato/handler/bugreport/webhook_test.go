package bugreport

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWebhookReport(t *testing.T) {
	var gotMethod, gotType string
	var raw []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotType = r.Header.Get("Content-Type")
		raw, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	h := &webhookReporter{client: srv.Client(), url: srv.URL}
	url, err := h.Report(context.Background(), Report{
		Pkgname: "foo", Severity: "high", Description: "boom",
		Email: "reporter@example.com", MaintainerEmail: "maintainer@secret.example",
	})
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

	// The server-resolved maintainer address must never leave the process.
	if strings.Contains(string(raw), "maintainer@secret.example") {
		t.Errorf("payload leaked MaintainerEmail: %s", raw)
	}

	var env map[string]json.RawMessage
	if err := json.Unmarshal(raw, &env); err != nil {
		t.Fatalf("unmarshal envelope: %v", err)
	}
	for _, k := range []string{"event", "id", "timestamp", "data"} {
		if _, ok := env[k]; !ok {
			t.Errorf("payload missing %q key: %s", k, raw)
		}
	}
	var event string
	_ = json.Unmarshal(env["event"], &event)
	if event != "bug_report" {
		t.Errorf("event = %q, want bug_report", event)
	}

	var data map[string]any
	if err := json.Unmarshal(env["data"], &data); err != nil {
		t.Fatalf("unmarshal data: %v", err)
	}
	for _, k := range []string{"pkgname", "pkgver", "name", "email", "severity", "description"} {
		if _, ok := data[k]; !ok {
			t.Errorf("data missing snake_case key %q: %s", k, env["data"])
		}
	}
	if _, ok := data["maintainer_email"]; ok {
		t.Error("data must not include maintainer_email")
	}
	if data["pkgname"] != "foo" || data["severity"] != "high" || data["description"] != "boom" {
		t.Errorf("data = %+v", data)
	}
	if data["email"] != "reporter@example.com" {
		t.Errorf("data email = %v, want reporter@example.com", data["email"])
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
