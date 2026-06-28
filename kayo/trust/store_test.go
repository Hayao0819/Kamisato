package trust

import (
	"path/filepath"
	"testing"
)

func TestStorePersistence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "trust.json")

	s, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	s.TrustMaintainer("aur", "jguer", "test")
	s.Approve(Approval{Pkgbase: "yay", Source: "aur", Maintainer: "jguer", Commit: "abc123"})
	if err := s.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	reopened, err := Open(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	if !reopened.IsMaintainerTrusted("aur", "jguer") {
		t.Error("maintainer not persisted")
	}
	// Per-source namespacing: same account name on a different source is distinct.
	if reopened.IsMaintainerTrusted("ayato-x", "jguer") {
		t.Error("trust must not leak across sources")
	}
	if _, ok := reopened.Approval("yay"); !ok {
		t.Error("approval not persisted")
	}
}

func TestEvaluate(t *testing.T) {
	s, _ := Open(filepath.Join(t.TempDir(), "trust.json"))
	s.Approve(Approval{Pkgbase: "yay", Source: "aur", Maintainer: "jguer", Commit: "c1"})

	cases := []struct {
		name               string
		source, base, main string
		want               Decision
	}{
		{"overlay always trusted", "overlay", "anything", "", Trusted},
		{"approved, same maintainer", "aur", "yay", "jguer", Trusted},
		{"unreviewed package", "aur", "newpkg", "someone", NeedsReview},
		{"maintainer changed (takeover)", "aur", "yay", "attacker", NeedsReview},
		{"orphaned", "aur", "yay", "", NeedsReview},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := s.Evaluate(c.source, c.base, c.main); got.Decision != c.want {
				t.Errorf("Evaluate = %v (%v), want %v", got.Decision, got.Reasons, c.want)
			}
		})
	}
}
