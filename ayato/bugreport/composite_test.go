package bugreport

import (
	"context"
	"errors"
	"testing"
)

type stubReporter struct {
	url    string
	err    error
	called bool
}

func (s *stubReporter) Report(_ context.Context, _ Report) (string, error) {
	s.called = true
	return s.url, s.err
}

func TestNewSingleAndComposite(t *testing.T) {
	single, err := New(Config{Backends: []string{"github"}, GitHub: GitHubConfig{Repo: "o/r", Token: "t"}})
	if err != nil {
		t.Fatalf("single backend: %v", err)
	}
	if _, ok := single.(*githubReporter); !ok {
		t.Errorf("one backend should return it directly, got %T", single)
	}

	multi, err := New(Config{
		Backends: []string{"github", "webhook"},
		GitHub:   GitHubConfig{Repo: "o/r", Token: "t"},
		Webhook:  WebhookConfig{URL: "https://example.com/hook"},
	})
	if err != nil {
		t.Fatalf("multi backend: %v", err)
	}
	if _, ok := multi.(*multiReporter); !ok {
		t.Errorf("several backends should compose, got %T", multi)
	}
}

func TestMultiReporterAllFail(t *testing.T) {
	a := &stubReporter{err: errors.New("a down")}
	b := &stubReporter{err: errors.New("b down")}
	m := &multiReporter{reporters: []Reporter{a, b}}
	if _, err := m.Report(context.Background(), Report{}); err == nil {
		t.Error("all backends failing must error")
	}
	if !a.called || !b.called {
		t.Error("every backend must be tried")
	}
}

func TestMultiReporterOneSucceeds(t *testing.T) {
	a := &stubReporter{err: errors.New("a down")}
	b := &stubReporter{url: "https://github.com/o/r/issues/1"}
	c := &stubReporter{} // succeeds with no url (smtp/webhook)
	m := &multiReporter{reporters: []Reporter{a, b, c}}
	url, err := m.Report(context.Background(), Report{})
	if err != nil {
		t.Fatalf("one success must not error: %v", err)
	}
	if url != "https://github.com/o/r/issues/1" {
		t.Errorf("url = %q, want the first non-empty success url", url)
	}
	if !a.called || !b.called || !c.called {
		t.Error("every backend must be tried even after a success")
	}
}
