package report

import (
	"testing"

	"github.com/Hayao0819/Kamisato/internal/client"
)

func TestLatestJobStatusPicksMostRecent(t *testing.T) {
	jobs := []client.Job{
		{Repo: "extra", Packages: []string{"foo"}, Status: "queued", CreatedAt: "2026-01-01T00:00:00Z"},
		{Repo: "extra", Packages: []string{"foo"}, Status: "success", CreatedAt: "2026-02-01T00:00:00Z"},
		{Repo: "other", Packages: []string{"foo"}, Status: "failed", CreatedAt: "2026-03-01T00:00:00Z"},
		// Whole-repo build (no packages listed) still matches.
		{Repo: "extra", Status: "running", CreatedAt: "2026-01-15T00:00:00Z"},
	}
	if got := LatestJobStatus(jobs, "extra", []string{"foo"}); got != "success" {
		t.Errorf("LatestJobStatus = %q, want success", got)
	}
	if s := LatestJobStatus(jobs, "extra", []string{"missing"}); s != "running" {
		// "missing" matches only the whole-repo build.
		t.Errorf("LatestJobStatus for whole-repo match = %q, want running", s)
	}
	if s := LatestJobStatus(jobs, "nope", []string{"foo"}); s != "" {
		t.Errorf("LatestJobStatus for unknown repo = %q, want empty", s)
	}
}
