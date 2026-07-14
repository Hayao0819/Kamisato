package bugreport

import (
	"slices"
	"strings"
	"testing"
)

func TestRecipients(t *testing.T) {
	cfg := SMTPConfig{To: "bugs@example.com", ToMaintainer: true}

	if got := recipients(cfg, Report{MaintainerEmail: "m@example.com"}); !slices.Equal(got, []string{"bugs@example.com", "m@example.com"}) {
		t.Errorf("with maintainer = %v", got)
	}
	if got := recipients(cfg, Report{}); !slices.Equal(got, []string{"bugs@example.com"}) {
		t.Errorf("maintainer enabled but unknown = %v, want To only", got)
	}

	off := SMTPConfig{To: "bugs@example.com"}
	if got := recipients(off, Report{MaintainerEmail: "m@example.com"}); !slices.Equal(got, []string{"bugs@example.com"}) {
		t.Errorf("maintainer disabled = %v, want To only", got)
	}
}

func TestBuildMessage(t *testing.T) {
	cfg := SMTPConfig{From: "noreply@example.com", To: "bugs@example.com", ToMaintainer: true}
	r := Report{
		Pkgname: "foo", Pkgver: "1.0-1", Severity: "high",
		Email: "reporter@example.com", Description: "boom", MaintainerEmail: "m@example.com",
	}
	m := buildMessage(cfg, r)

	if m.From != "noreply@example.com" {
		t.Errorf("from = %q", m.From)
	}
	if !slices.Equal(m.To, []string{"bugs@example.com", "m@example.com"}) {
		t.Errorf("to = %v", m.To)
	}
	if m.ReplyTo != "reporter@example.com" {
		t.Errorf("reply-to = %q, want the reporter", m.ReplyTo)
	}
	if !strings.Contains(m.Subject, "foo") || !strings.Contains(m.Subject, "high") {
		t.Errorf("subject = %q, want pkg + severity", m.Subject)
	}
	if !strings.Contains(m.Body, "boom") {
		t.Errorf("body = %q, want the description", m.Body)
	}
}

func TestNewSMTPValidation(t *testing.T) {
	if _, err := newSMTP(SMTPConfig{From: "a@b", To: "c@d"}); err == nil {
		t.Error("smtp without a host must error")
	}
	if _, err := newSMTP(SMTPConfig{Host: "smtp", To: "c@d"}); err == nil {
		t.Error("smtp without a from must error")
	}
	if _, err := newSMTP(SMTPConfig{Host: "smtp", From: "a@b"}); err == nil {
		t.Error("smtp with neither to nor to_maintainer must error")
	}
	if _, err := newSMTP(SMTPConfig{Host: "smtp", From: "a@b", To: "c@d"}); err != nil {
		t.Errorf("valid smtp config: %v", err)
	}
}
