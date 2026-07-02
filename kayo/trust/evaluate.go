package trust

import "fmt"

// Decision is the resolution-layer trust outcome (the maintainer-account check;
// the commit pin is verified separately at build time).
type Decision int

const (
	Trusted Decision = iota
	NeedsReview
)

func (d Decision) String() string {
	if d == Trusted {
		return "trusted"
	}
	return "needs-review"
}

type Verdict struct {
	Decision Decision
	Reasons  []string
}

// EvaluateResolved is the canonical trust verdict for a package already resolved
// through the federation layer. A delegated source whose signed catalog currently
// verifies is trusted outright, bypassing the store; a nil store is likewise
// trusting (gating disabled). Otherwise Evaluate decides. The federation merge
// gate and the install-time verify hook both route through this so the delegation
// bypass cannot drift between them.
func (s *Store) EvaluateResolved(source, pkgbase, maintainer string, delegatedVerified bool) Verdict {
	if delegatedVerified || s == nil {
		return Verdict{Decision: Trusted}
	}
	return s.Evaluate(source, pkgbase, maintainer)
}

// Evaluate judges a resolved package using only resolution-time facts (source,
// pkgbase, current maintainer account). It does not look at the commit — that is
// the build-time pin's job. Overlays are trusted by configuration and should be
// passed source "overlay". A whitelisted pkgbase is auto-approved regardless of
// review state. A vouched maintainer (TrustMaintainer) auto-allows a HANDOFF of an
// already-approved package to that account; it never auto-trusts a brand-new,
// unreviewed package.
func (s *Store) Evaluate(source, pkgbase, maintainer string) Verdict {
	if source == "overlay" {
		return Verdict{Decision: Trusted}
	}
	// An explicit whitelist entry is a deliberate blanket trust of the pkgbase; it
	// skips both the new-package inspection below and the maintainer-change check.
	if s.IsWhitelisted(pkgbase) {
		return Verdict{Decision: Trusted}
	}

	ap, ok := s.Approval(pkgbase)
	if !ok {
		return Verdict{Decision: NeedsReview, Reasons: []string{"unreviewed package"}}
	}
	// A changed maintainer account is the takeover/adoption signal — including a
	// maintained package going orphaned (jguer -> ""). Sources without an account
	// (local/url) record "" at both ends and stay trusted on the commit pin.
	if ap.Maintainer != maintainer {
		// Adoption by an account the user vouches for is a sanctioned handoff.
		if maintainer != "" && s.IsMaintainerTrusted(source, maintainer) {
			return Verdict{Decision: Trusted}
		}
		return Verdict{Decision: NeedsReview, Reasons: []string{
			fmt.Sprintf("maintainer changed: %q -> %q", ap.Maintainer, maintainer),
		}}
	}
	return Verdict{Decision: Trusted}
}
