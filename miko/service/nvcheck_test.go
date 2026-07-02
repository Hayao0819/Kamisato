package service

import (
	"testing"

	"github.com/Hayao0819/Kamisato/internal/conf"
	"github.com/Hayao0819/Kamisato/miko/domain"
	"github.com/Hayao0819/Kamisato/miko/nvcheck"
)

// A monitored rebuild must enqueue a real job tagged ReasonVersionUpdate so its
// origin is visible, reusing Submit's validation.
func TestVersionUpdateEnqueuerTagsReason(t *testing.T) {
	s := New(&conf.MikoConfig{}, nil, nil, nil).(*Service)
	enq := &versionUpdateEnqueuer{s: s}

	entry := nvcheck.Entry{Pkgbase: "foo", Repo: "extra", Arch: "x86_64", Git: "https://aur.archlinux.org/foo.git"}
	if err := enq.EnqueueVersionUpdate(entry, "2.0.0"); err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	jobs := s.List()
	if len(jobs) != 1 {
		t.Fatalf("want 1 job, got %d", len(jobs))
	}
	if jobs[0].Reason != domain.ReasonVersionUpdate {
		t.Errorf("Reason = %q, want %q", jobs[0].Reason, domain.ReasonVersionUpdate)
	}
	if jobs[0].Arch != "x86_64" || jobs[0].Repo != "extra" {
		t.Errorf("job target = %s/%s, want extra/x86_64", jobs[0].Repo, jobs[0].Arch)
	}
}

// nvcheckEntries fills the clone URL from aur_git_base when an entry omits it.
func TestNvcheckEntriesDefaultsGitURL(t *testing.T) {
	cfg := &conf.MikoConfig{AURGitBase: "https://aur.archlinux.org"}
	cfg.NvCheck.Entries = []conf.NvCheckEntry{
		{Pkgbase: "foo", Kind: "github", Repo: "o/foo"},
		{Pkgbase: "bar", Kind: "pypi", Package: "bar", Git: "https://example.com/bar.git"},
	}
	got := nvcheckEntries(cfg)
	if got[0].Git != "https://aur.archlinux.org/foo.git" {
		t.Errorf("default git = %q", got[0].Git)
	}
	if got[1].Git != "https://example.com/bar.git" {
		t.Errorf("explicit git overridden: %q", got[1].Git)
	}
}
