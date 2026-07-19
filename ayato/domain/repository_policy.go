package domain

import (
	"fmt"
	"slices"

	"github.com/Hayao0819/Kamisato/pkg/pacman/reponame"
)

// Tier names a stage in a tiered repository's promotion flow.
type Tier string

const (
	TierStaging Tier = "staging"
	TierTesting Tier = "testing"
	TierStable  Tier = "stable"
)

func ParseTier(value string) (Tier, error) {
	tier := Tier(value)
	switch tier {
	case TierStaging, TierTesting, TierStable:
		return tier, nil
	default:
		return "", fmt.Errorf("%w: unknown repository tier %q", ErrInvalid, value)
	}
}

func CanPromote(from, to Tier) bool {
	return from == TierStaging && to == TierTesting ||
		from == TierTesting && to == TierStable
}

// RepositorySpec is the configuration-independent input for one logical
// repository policy.
type RepositorySpec struct {
	Name                  string
	Arches                []string
	AllowNewArch          bool
	TrustedKeys           []string
	Tiered                bool
	PromotionKeepInSource bool
	Upstream              UpstreamSpec
}

// Repository is one validated logical policy. Slice accessors always clone.
type Repository struct {
	name                  string
	arches                []string
	allowNewArch          bool
	trustedKeys           []string
	tiered                bool
	promotionKeepInSource bool
	upstream              Upstream
}

func newRepository(
	defaultArches []string,
	spec RepositorySpec,
) (*Repository, error) {
	if err := reponame.Validate(spec.Name); err != nil {
		return nil, err
	}
	arches := spec.Arches
	if len(arches) == 0 {
		arches = defaultArches
	}
	normalizedArches, err := validateArches(arches)
	if err != nil {
		return nil, fmt.Errorf("repository %q: %w", spec.Name, err)
	}
	upstream, err := newUpstream(spec.Upstream)
	if err != nil {
		return nil, fmt.Errorf("repository %q: %w", spec.Name, err)
	}
	return &Repository{
		name:                  spec.Name,
		arches:                normalizedArches,
		allowNewArch:          spec.AllowNewArch,
		trustedKeys:           slices.Clone(spec.TrustedKeys),
		tiered:                spec.Tiered,
		promotionKeepInSource: spec.PromotionKeepInSource,
		upstream:              upstream,
	}, nil
}

func validateArches(arches []string) ([]string, error) {
	result := make([]string, 0, len(arches))
	seen := make(map[string]struct{}, len(arches))
	for _, arch := range arches {
		if err := validateArch(arch); err != nil {
			return nil, err
		}
		if _, duplicate := seen[arch]; duplicate {
			return nil, fmt.Errorf("declared architecture %q is duplicated", arch)
		}
		seen[arch] = struct{}{}
		result = append(result, arch)
	}
	return result, nil
}

func validateArch(arch string) error {
	if arch == "" {
		return fmt.Errorf("declared architecture must not be empty")
	}
	if arch == "any" {
		return fmt.Errorf(
			"declared architecture %q has no pacman repository database",
			arch,
		)
	}
	for _, character := range arch {
		if !isArchCharacter(character) {
			return fmt.Errorf(
				"declared architecture %q contains unsupported character %q",
				arch,
				character,
			)
		}
	}
	return nil
}

func isArchCharacter(character rune) bool {
	return character >= 'a' && character <= 'z' ||
		character >= 'A' && character <= 'Z' ||
		character >= '0' && character <= '9' ||
		character == '_' || character == '-' ||
		character == '.' || character == '+'
}

func (r *Repository) Name() string               { return r.name }
func (r *Repository) Arches() []string           { return slices.Clone(r.arches) }
func (r *Repository) AllowsNewArch() bool        { return r.allowNewArch }
func (r *Repository) TrustedKeys() []string      { return slices.Clone(r.trustedKeys) }
func (r *Repository) Tiered() bool               { return r.tiered }
func (r *Repository) PromotionKeepsSource() bool { return r.promotionKeepInSource }
func (r *Repository) Upstream() Upstream         { return r.upstream }

func (r *Repository) PhysicalName(tier Tier) (string, error) {
	if _, err := ParseTier(string(tier)); err != nil {
		return "", err
	}
	if !r.tiered && tier != TierStable {
		return "", fmt.Errorf(
			"%w: repository %q is not tiered",
			ErrInvalid,
			r.name,
		)
	}
	if tier == TierStable {
		return r.name, nil
	}
	return r.name + "-" + string(tier), nil
}

func (r *Repository) PhysicalNames() []string {
	names := make([]string, 0, len(r.tiers()))
	for _, tier := range r.tiers() {
		physical, _ := r.PhysicalName(tier)
		names = append(names, physical)
	}
	return names
}

func (r *Repository) tiers() []Tier {
	if !r.tiered {
		return []Tier{TierStable}
	}
	return []Tier{TierStaging, TierTesting, TierStable}
}
