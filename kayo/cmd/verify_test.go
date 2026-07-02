package cmd

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Hayao0819/Kamisato/kayo/trust"
	"github.com/Hayao0819/Kamisato/pkg/aurweb"
)

// statusByName parses reportTrust output into name -> status token ("ok"/"REVIEW").
func statusByName(out string) map[string]string {
	m := map[string]string{}
	for _, ln := range strings.Split(strings.TrimSpace(out), "\n") {
		if f := strings.Fields(ln); len(f) >= 2 {
			m[f[1]] = f[0]
		}
	}
	return m
}

// TestReportTrust is the install-time pacman-hook gate: it must pass a reviewed
// package, flag an unreviewed one and a maintainer-change takeover for review, and
// skip targets it does not manage (official-repo packages).
func TestReportTrust(t *testing.T) {
	store, err := trust.Open(filepath.Join(t.TempDir(), "trust.json"))
	if err != nil {
		t.Fatal(err)
	}
	store.Approve(trust.Approval{Pkgbase: "yay", Source: "aur", Maintainer: "jguer", Commit: "c1"})
	store.Approve(trust.Approval{Pkgbase: "paru", Source: "aur", Maintainer: "morganamilo", Commit: "c2"})

	found := map[string]verifyTarget{
		"yay":    {pkg: aurweb.Pkg{Name: "yay", PackageBase: "yay", Maintainer: "jguer"}, source: "aur"},
		"newpkg": {pkg: aurweb.Pkg{Name: "newpkg", PackageBase: "newpkg", Maintainer: "someone"}, source: "aur"},
		"paru":   {pkg: aurweb.Pkg{Name: "paru", PackageBase: "paru", Maintainer: "attacker"}, source: "aur"},
		"local":  {pkg: aurweb.Pkg{Name: "local", PackageBase: "local"}, source: "overlay"},
	}
	// "official" is not in found: an official-repo target the hook must ignore.
	order := []string{"yay", "newpkg", "paru", "local", "official"}

	var buf bytes.Buffer
	needsReview := reportTrust(&buf, store, order, found)
	if !needsReview {
		t.Error("needsReview must be true when a target needs review (enforce would block)")
	}
	got := statusByName(buf.String())

	for name, want := range map[string]string{
		"yay":    "ok",     // reviewed, unchanged maintainer
		"newpkg": "REVIEW", // never reviewed
		"paru":   "REVIEW", // approved but maintainer changed (takeover)
		"local":  "ok",     // overlays are trusted by config
	} {
		if got[name] != want {
			t.Errorf("target %q: status %q, want %q (out=%q)", name, got[name], want, buf.String())
		}
	}
	if _, ok := got["official"]; ok {
		t.Errorf("official-repo target must be skipped, got a status line (out=%q)", buf.String())
	}
	if !strings.Contains(buf.String(), "maintainer changed") {
		t.Errorf("takeover line should explain the maintainer change: %q", buf.String())
	}
}

// TestReportTrustDelegatedBypass locks in the security-load-bearing bypass: a
// delegated source whose attestation currently verifies passes an unreviewed
// package, but the identical package without that flag is held for review. The
// bypass must depend ONLY on the verified delegation, never leak to plain sources.
func TestReportTrustDelegatedBypass(t *testing.T) {
	store, err := trust.Open(filepath.Join(t.TempDir(), "trust.json")) // empty: nothing reviewed
	if err != nil {
		t.Fatal(err)
	}
	pkg := aurweb.Pkg{Name: "brand-new", PackageBase: "brand-new", Maintainer: "who"}

	var deleg bytes.Buffer
	if reportTrust(&deleg, store, []string{"brand-new"},
		map[string]verifyTarget{"brand-new": {pkg: pkg, source: "mirror", delegatedVerified: true}}) {
		t.Error("a delegated-verified target must pass without review")
	}
	if statusByName(deleg.String())["brand-new"] != "ok" {
		t.Errorf("delegated-verified target should print ok: %q", deleg.String())
	}

	var plain bytes.Buffer
	if !reportTrust(&plain, store, []string{"brand-new"},
		map[string]verifyTarget{"brand-new": {pkg: pkg, source: "mirror"}}) {
		t.Error("the same target without the verified delegation must be held for review")
	}
	if statusByName(plain.String())["brand-new"] != "REVIEW" {
		t.Errorf("non-delegated unreviewed target should print REVIEW: %q", plain.String())
	}
}
