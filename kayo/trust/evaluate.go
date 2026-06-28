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

// Evaluate judges a resolved package using only resolution-time facts (source,
// pkgbase, current maintainer account). It does not look at the commit — that is
// the build-time pin's job. Overlays are trusted by configuration and should be
// passed source "overlay".
func (s *Store) Evaluate(source, pkgbase, maintainer string) Verdict {
	if source == "overlay" {
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
		return Verdict{Decision: NeedsReview, Reasons: []string{
			fmt.Sprintf("maintainer changed: %q -> %q", ap.Maintainer, maintainer),
		}}
	}
	return Verdict{Decision: Trusted}
}
