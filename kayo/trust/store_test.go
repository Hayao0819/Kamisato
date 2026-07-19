package trust

import (
	"path/filepath"
	"testing"
)

type evaluationCase struct {
	name               string
	source, base, main string
	want               Decision
}

func assertEvaluations(t *testing.T, store *Store, cases []evaluationCase) {
	t.Helper()
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := store.Evaluate(c.source, c.base, c.main); got.Decision != c.want {
				t.Errorf("Evaluate = %v (%v), want %v", got.Decision, got.Reasons, c.want)
			}
		})
	}
}

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

	assertEvaluations(t, s, []evaluationCase{
		{"overlay always trusted", "overlay", "anything", "", Trusted},
		{"approved, same maintainer", "aur", "yay", "jguer", Trusted},
		{"unreviewed package", "aur", "newpkg", "someone", NeedsReview},
		{"maintainer changed (takeover)", "aur", "yay", "attacker", NeedsReview},
		{"orphaned", "aur", "yay", "", NeedsReview},
	})
}

func TestEvaluateWhitelist(t *testing.T) {
	s, _ := Open(filepath.Join(t.TempDir(), "trust.json"))
	s.Approve(Approval{Pkgbase: "yay", Source: "aur", Maintainer: "jguer", Commit: "c1"})
	s.AddWhitelist("firefox", "vendored, reviewed out-of-band")

	assertEvaluations(t, s, []evaluationCase{
		{"whitelisted brand-new pkgbase is trusted", "aur", "firefox", "anyone", Trusted},
		{"non-whitelisted brand-new pkgbase needs review", "aur", "newpkg", "someone", NeedsReview},
		{"non-whitelisted changed maintainer needs review", "aur", "yay", "attacker", NeedsReview},
		{"non-whitelisted unchanged is trusted", "aur", "yay", "jguer", Trusted},
	})

	// A whitelist entry bypasses maintainer-change detection even for a pkgbase that
	// also has an approval, so removing it restores the maintainer-swap gate.
	s.AddWhitelist("yay", "")
	if got := s.Evaluate("aur", "yay", "attacker"); got.Decision != Trusted {
		t.Errorf("whitelisted yay = %v, want Trusted", got.Decision)
	}
	s.RemoveWhitelist("yay")
	if got := s.Evaluate("aur", "yay", "attacker"); got.Decision != NeedsReview {
		t.Errorf("un-whitelisted yay takeover = %v, want NeedsReview", got.Decision)
	}
}

func TestWhitelistPersistence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "trust.json")
	s, _ := Open(path)
	s.AddWhitelist("firefox", "note")
	if err := s.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}
	reopened, err := Open(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	if !reopened.IsWhitelisted("firefox") {
		t.Error("whitelist entry not persisted")
	}
	if got := reopened.WhitelistEntries(); len(got) != 1 || got[0].Pkgbase != "firefox" {
		t.Errorf("WhitelistEntries = %+v, want one firefox entry", got)
	}
}

func TestEvaluateVouchedAdoption(t *testing.T) {
	s, _ := Open(filepath.Join(t.TempDir(), "trust.json"))
	s.Approve(Approval{Pkgbase: "yay", Source: "aur", Maintainer: "jguer", Commit: "c1"})
	s.TrustMaintainer("aur", "successor", "")

	// Handoff of an approved package to a vouched account is sanctioned.
	if got := s.Evaluate("aur", "yay", "successor"); got.Decision != Trusted {
		t.Errorf("vouched adoption = %v (%v), want Trusted", got.Decision, got.Reasons)
	}
	// Handoff to an account we do not vouch for still needs review.
	if got := s.Evaluate("aur", "yay", "attacker"); got.Decision != NeedsReview {
		t.Errorf("non-vouched takeover = %v, want NeedsReview", got.Decision)
	}
	// A brand-new package by a vouched account is not auto-trusted.
	if got := s.Evaluate("aur", "newpkg", "successor"); got.Decision != NeedsReview {
		t.Errorf("new pkg by vouched account = %v, want NeedsReview", got.Decision)
	}
}
