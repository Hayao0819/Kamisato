package domain

import "strings"

// AURTrustPolicySpec is the configuration-independent input for the AUR
// dependency trust policy.
type AURTrustPolicySpec struct {
	TrustedMaintainers []string
	TrustedPkgbases    []string
	AllowUntrusted     bool
}

// AURTrustDecision explains why a resolved dependency may or may not build.
type AURTrustDecision uint8

const (
	// AURTrustBlocked means the dependency is neither trusted nor explicitly
	// allowed. This is the secure zero value.
	AURTrustBlocked AURTrustDecision = iota
	// AURTrustByPkgbase means the pkgbase is on the explicit allowlist.
	AURTrustByPkgbase
	// AURTrustByMaintainer means the current non-empty maintainer is trusted.
	AURTrustByMaintainer
	// AURTrustUntrusted means permissive policy explicitly allows the dependency.
	AURTrustUntrusted
)

// AURTrustPolicy is an immutable snapshot of the dependency trust rules.
type AURTrustPolicy struct {
	trustedMaintainers map[string]struct{}
	trustedPkgbases    map[string]struct{}
	allowUntrusted     bool
}

// NewAURTrustPolicy normalizes maintainer identities once and copies all input,
// so later config mutation cannot change a running service's trust boundary.
func NewAURTrustPolicy(spec AURTrustPolicySpec) AURTrustPolicy {
	policy := AURTrustPolicy{
		trustedMaintainers: make(map[string]struct{}, len(spec.TrustedMaintainers)),
		trustedPkgbases:    make(map[string]struct{}, len(spec.TrustedPkgbases)),
		allowUntrusted:     spec.AllowUntrusted,
	}
	for _, maintainer := range spec.TrustedMaintainers {
		if normalized := normalizeMaintainer(maintainer); normalized != "" {
			policy.trustedMaintainers[normalized] = struct{}{}
		}
	}
	for _, pkgbase := range spec.TrustedPkgbases {
		if pkgbase != "" {
			policy.trustedPkgbases[pkgbase] = struct{}{}
		}
	}
	return policy
}

// Decide applies trust in strongest-to-weakest order. An empty maintainer is
// orphaned and never gains maintainer trust; it can pass only through the
// pkgbase allowlist or the explicit permissive policy.
func (p AURTrustPolicy) Decide(pkgbase, maintainer string) AURTrustDecision {
	if _, trusted := p.trustedPkgbases[pkgbase]; trusted {
		return AURTrustByPkgbase
	}
	if normalized := normalizeMaintainer(maintainer); normalized != "" {
		if _, trusted := p.trustedMaintainers[normalized]; trusted {
			return AURTrustByMaintainer
		}
	}
	if p.allowUntrusted {
		return AURTrustUntrusted
	}
	return AURTrustBlocked
}

func normalizeMaintainer(maintainer string) string {
	return strings.ToLower(strings.TrimSpace(maintainer))
}
