package service

import (
	"context"
	stderrors "errors"
	"path/filepath"
	"strings"
	"testing"

	kayoconfig "github.com/Hayao0819/Kamisato/kayo/config"
	"github.com/Hayao0819/Kamisato/kayo/trust"
	"github.com/Hayao0819/Kamisato/pkg/aurweb"
)

// TestEvaluateTrust is the install-time pacman-hook gate: it must pass a reviewed
// package, flag an unreviewed one and a maintainer-change takeover for review, and
// skip targets it does not manage (official-repo packages).
func TestEvaluateTrust(t *testing.T) {
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

	results, needsReview := evaluateTrust(store, order, found)
	if !needsReview {
		t.Error("needsReview must be true when a target needs review (enforce would block)")
	}
	got := make(map[string]trust.Verdict, len(results))
	for _, result := range results {
		got[result.Name] = result.Verdict
	}

	for name, want := range map[string]trust.Decision{
		"yay":    trust.Trusted,
		"newpkg": trust.NeedsReview,
		"paru":   trust.NeedsReview,
		"local":  trust.Trusted,
	} {
		if got[name].Decision != want {
			t.Errorf("target %q: decision %q, want %q", name, got[name].Decision, want)
		}
	}
	if _, ok := got["official"]; ok {
		t.Errorf("official-repo target must be skipped")
	}
	if !strings.Contains(strings.Join(got["paru"].Reasons, "; "), "maintainer changed") {
		t.Errorf("takeover decision should explain the maintainer change: %+v", got["paru"])
	}
}

// TestReportTrustDelegatedBypass locks in the security-load-bearing bypass: a
// delegated source whose attestation currently verifies passes an unreviewed
// package, but the identical package without that flag is held for review. The
// bypass must depend ONLY on the verified delegation, never leak to plain sources.
func TestEvaluateTrustDelegatedBypass(t *testing.T) {
	store, err := trust.Open(filepath.Join(t.TempDir(), "trust.json")) // empty: nothing reviewed
	if err != nil {
		t.Fatal(err)
	}
	pkg := aurweb.Pkg{Name: "brand-new", PackageBase: "brand-new", Maintainer: "who"}

	deleg, needsReview := evaluateTrust(store, []string{"brand-new"},
		map[string]verifyTarget{"brand-new": {pkg: pkg, source: "mirror", delegatedVerified: true}})
	if needsReview {
		t.Error("a delegated-verified target must pass without review")
	}
	if len(deleg) != 1 || deleg[0].Verdict.Decision != trust.Trusted {
		t.Errorf("delegated-verified target should be trusted: %+v", deleg)
	}

	plain, needsReview := evaluateTrust(store, []string{"brand-new"},
		map[string]verifyTarget{"brand-new": {pkg: pkg, source: "mirror"}})
	if !needsReview {
		t.Error("the same target without the verified delegation must be held for review")
	}
	if len(plain) != 1 || plain[0].Verdict.Decision != trust.NeedsReview {
		t.Errorf("non-delegated unreviewed target should need review: %+v", plain)
	}
}

func TestVerifyFailsClosedWhenUpstreamLookupFails(t *testing.T) {
	lookupErr := stderrors.New("upstream unavailable")
	deps := defaultVerifyDependencies()
	deps.newResolver = func(context.Context, *kayoconfig.KayoConfig) (resolvePackage, error) {
		return func(context.Context, string) (aurweb.Pkg, string, bool, bool) {
			return aurweb.Pkg{}, "", false, false
		}, nil
	}
	deps.newUpstream = func(*kayoconfig.KayoConfig) lookupPackages {
		return func(context.Context, []string) ([]aurweb.Pkg, error) {
			return nil, lookupErr
		}
	}
	deps.syncPackages = func() (map[string]bool, error) {
		return map[string]bool{}, nil
	}

	cfg := &kayoconfig.KayoConfig{
		TrustStore:  filepath.Join(t.TempDir(), "trust.json"),
		EnforceMode: "enforce",
	}
	_, err := verify(deps, context.Background(), cfg, []string{"foreign"}, false)
	if !stderrors.Is(err, lookupErr) {
		t.Fatalf("error = %v, want upstream lookup error", err)
	}
}

func TestVerifySkipsUpstreamForOfficialPackages(t *testing.T) {
	deps := defaultVerifyDependencies()
	deps.newResolver = func(context.Context, *kayoconfig.KayoConfig) (resolvePackage, error) {
		return func(context.Context, string) (aurweb.Pkg, string, bool, bool) {
			return aurweb.Pkg{}, "", false, false
		}, nil
	}
	lookups := 0
	deps.newUpstream = func(*kayoconfig.KayoConfig) lookupPackages {
		return func(context.Context, []string) ([]aurweb.Pkg, error) {
			lookups++
			return nil, nil
		}
	}
	deps.syncPackages = func() (map[string]bool, error) {
		return map[string]bool{"official": true}, nil
	}

	cfg := &kayoconfig.KayoConfig{TrustStore: filepath.Join(t.TempDir(), "trust.json")}
	if _, err := verify(deps, context.Background(), cfg, []string{"official"}, false); err != nil {
		t.Fatal(err)
	}
	if lookups != 0 {
		t.Errorf("upstream lookups = %d, want 0", lookups)
	}
}
