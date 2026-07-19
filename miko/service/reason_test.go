package service

import (
	"testing"

	"github.com/Hayao0819/Kamisato/internal/conf"
	"github.com/Hayao0819/Kamisato/miko/domain"
)

// A user submission is recorded as a manual build so its origin shows in status.
func TestSubmitTagsReasonManual(t *testing.T) {
	s := New(&conf.MikoConfig{})

	id, err := s.Submit(&domain.BuildRequest{Arch: "x86_64", Pkgbuild: "pkgname=foo"})
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	job, err := s.Status(id)
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if job.Reason != domain.ReasonManual {
		t.Errorf("Reason = %q, want %q", job.Reason, domain.ReasonManual)
	}
}
