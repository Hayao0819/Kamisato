package domain

import (
	"fmt"
	"net/url"
	"slices"
	"strings"

	"github.com/Hayao0819/Kamisato/pkg/pacman/reponame"
)

// Tier names a stage in a tiered repository's promotion flow.
type Tier string

const (
	TierStaging Tier = "staging"
	TierTesting Tier = "testing"
	TierStable  Tier = "stable"
)

// ParseTier parses a transport/config string without allowing an unknown value
// to enter the domain as a Tier.
func ParseTier(value string) (Tier, error) {
	tier := Tier(value)
	switch tier {
	case TierStaging, TierTesting, TierStable:
		return tier, nil
	default:
		return "", fmt.Errorf("%w: unknown repository tier %q", ErrInvalid, value)
	}
}

// CanPromote reports whether from -> to is a legal single promotion step.
func CanPromote(from, to Tier) bool {
	return from == TierStaging && to == TierTesting ||
		from == TierTesting && to == TierStable
}

// UpstreamSpec is the configuration-independent input for an upstream overlay.
type UpstreamSpec struct {
	DBURL    string
	FilesURL string
}

// RepositorySpec is the configuration-independent input used to construct a
// catalog entry.
type RepositorySpec struct {
	Name                  string
	Arches                []string
	AllowNewArch          bool
	TrustedKeys           []string
	Tiered                bool
	PromotionKeepInSource bool
	Upstream              UpstreamSpec
}

// Upstream is a validated upstream overlay definition.
type Upstream struct {
	dbURL    string
	filesURL string
}

func newUpstream(spec UpstreamSpec) (Upstream, error) {
	if spec.DBURL == "" {
		if spec.FilesURL != "" {
			return Upstream{}, fmt.Errorf("upstream files_url requires db_url")
		}
		return Upstream{}, nil
	}
	if err := validateUpstreamURL("db_url", spec.DBURL); err != nil {
		return Upstream{}, err
	}
	if spec.FilesURL != "" {
		if err := validateUpstreamURL("files_url", spec.FilesURL); err != nil {
			return Upstream{}, err
		}
	}
	return Upstream{dbURL: spec.DBURL, filesURL: spec.FilesURL}, nil
}

func validateUpstreamURL(field, value string) error {
	expanded := strings.ReplaceAll(value, "$arch", "x86_64")
	u, err := url.Parse(expanded)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return fmt.Errorf("upstream %s must be an absolute http(s) URL with a host: %q", field, value)
	}
	return nil
}

// Enabled reports whether the repository layers an upstream database.
func (u Upstream) Enabled() bool { return u.dbURL != "" }

// DBURLFor resolves the upstream database URL for one architecture.
func (u Upstream) DBURLFor(arch string) string {
	return strings.ReplaceAll(u.dbURL, "$arch", arch)
}

// FilesURLFor resolves the upstream files database URL for one architecture.
func (u Upstream) FilesURLFor(arch string) string {
	filesURL := u.filesURL
	if filesURL == "" {
		filesURL = deriveFilesURL(u.dbURL)
	}
	return strings.ReplaceAll(filesURL, "$arch", arch)
}

func deriveFilesURL(dbURL string) string {
	i := strings.LastIndex(dbURL, ".db")
	if i < 0 {
		return dbURL
	}
	return dbURL[:i] + ".files" + dbURL[i+len(".db"):]
}

// Repository is one validated logical repository. Its slices are never exposed
// without cloning, keeping a catalog stable after construction.
type Repository struct {
	name                  string
	arches                []string
	allowNewArch          bool
	trustedKeys           []string
	tiered                bool
	promotionKeepInSource bool
	upstream              Upstream
}

func newRepository(defaultArches []string, spec RepositorySpec) (*Repository, error) {
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
	out := make([]string, 0, len(arches))
	seen := make(map[string]struct{}, len(arches))
	for _, arch := range arches {
		if arch == "" {
			return nil, fmt.Errorf("declared architecture must not be empty")
		}
		if arch == "any" {
			return nil, fmt.Errorf("declared architecture %q has no pacman repository database", arch)
		}
		for _, r := range arch {
			if isArchCharacter(r) {
				continue
			}
			return nil, fmt.Errorf("declared architecture %q contains unsupported character %q", arch, r)
		}
		if _, duplicate := seen[arch]; duplicate {
			return nil, fmt.Errorf("declared architecture %q is duplicated", arch)
		}
		seen[arch] = struct{}{}
		out = append(out, arch)
	}
	return out, nil
}

func isArchCharacter(r rune) bool {
	return r >= 'a' && r <= 'z' ||
		r >= 'A' && r <= 'Z' ||
		r >= '0' && r <= '9' ||
		r == '_' || r == '-' || r == '.' || r == '+'
}

func (r *Repository) Name() string               { return r.name }
func (r *Repository) Arches() []string           { return slices.Clone(r.arches) }
func (r *Repository) AllowsNewArch() bool        { return r.allowNewArch }
func (r *Repository) TrustedKeys() []string      { return slices.Clone(r.trustedKeys) }
func (r *Repository) Tiered() bool               { return r.tiered }
func (r *Repository) PromotionKeepsSource() bool { return r.promotionKeepInSource }
func (r *Repository) Upstream() Upstream         { return r.upstream }

// PhysicalName returns the pacman repository name that serves tier. A
// non-tiered repository has only its stable (bare-name) tier.
func (r *Repository) PhysicalName(tier Tier) (string, error) {
	if _, err := ParseTier(string(tier)); err != nil {
		return "", err
	}
	if !r.tiered && tier != TierStable {
		return "", fmt.Errorf("%w: repository %q is not tiered", ErrInvalid, r.name)
	}
	if tier == TierStable {
		return r.name, nil
	}
	return r.name + "-" + string(tier), nil
}

// PhysicalNames returns all pacman repository names served by this logical
// repository in promotion order.
func (r *Repository) PhysicalNames() []string {
	if !r.tiered {
		return []string{r.name}
	}
	return []string{
		r.name + "-" + string(TierStaging),
		r.name + "-" + string(TierTesting),
		r.name,
	}
}

// ResolvedRepository identifies both the logical policy and the concrete
// physical repository selected by a name.
type ResolvedRepository struct {
	Repository *Repository
	Physical   string
	Tier       Tier
}

// RepositoryCatalog is the single source of truth for logical repositories,
// physical tier names, architecture admission, and upstream policy.
type RepositoryCatalog struct {
	logicalOrder  []string
	physicalOrder []string
	logical       map[string]*Repository
	physical      map[string]ResolvedRepository
}

// NewRepositoryCatalog validates the complete topology. In particular, it
// rejects physical-name collisions such as tiered "core" plus a separately
// configured "core-staging".
func NewRepositoryCatalog(defaultArches []string, specs []RepositorySpec) (*RepositoryCatalog, error) {
	defaults, err := validateArches(defaultArches)
	if err != nil {
		return nil, fmt.Errorf("default_arches: %w", err)
	}
	catalog := &RepositoryCatalog{
		logical:  make(map[string]*Repository, len(specs)),
		physical: make(map[string]ResolvedRepository, len(specs)),
	}
	for _, spec := range specs {
		repository, err := newRepository(defaults, spec)
		if err != nil {
			return nil, err
		}
		if _, duplicate := catalog.logical[repository.name]; duplicate {
			return nil, fmt.Errorf("logical repository name %q is duplicated", repository.name)
		}
		catalog.logical[repository.name] = repository
		catalog.logicalOrder = append(catalog.logicalOrder, repository.name)

		tiers := []Tier{TierStable}
		if repository.tiered {
			tiers = []Tier{TierStaging, TierTesting, TierStable}
		}
		for _, tier := range tiers {
			physical, _ := repository.PhysicalName(tier)
			if previous, collision := catalog.physical[physical]; collision {
				return nil, fmt.Errorf(
					"physical repository name %q collides between logical repositories %q and %q",
					physical,
					previous.Repository.name,
					repository.name,
				)
			}
			catalog.physical[physical] = ResolvedRepository{
				Repository: repository,
				Physical:   physical,
				Tier:       tier,
			}
			catalog.physicalOrder = append(catalog.physicalOrder, physical)
		}
	}
	return catalog, nil
}

// Logical resolves only a configured logical name.
func (c *RepositoryCatalog) Logical(name string) (*Repository, bool) {
	if c == nil {
		return nil, false
	}
	repository, ok := c.logical[name]
	return repository, ok
}

// Resolve maps a physical pacman repository name to its logical policy and tier.
func (c *RepositoryCatalog) Resolve(name string) (ResolvedRepository, bool) {
	if c == nil {
		return ResolvedRepository{}, false
	}
	resolved, ok := c.physical[name]
	return resolved, ok
}

func (c *RepositoryCatalog) LogicalNames() []string {
	if c == nil {
		return nil
	}
	return slices.Clone(c.logicalOrder)
}

func (c *RepositoryCatalog) PhysicalNames() []string {
	if c == nil {
		return nil
	}
	return slices.Clone(c.physicalOrder)
}

// UpstreamPhysicalNames returns every physical repository whose logical policy
// layers an upstream database.
func (c *RepositoryCatalog) UpstreamPhysicalNames() []string {
	if c == nil {
		return nil
	}
	out := make([]string, 0, len(c.physicalOrder))
	for _, name := range c.physicalOrder {
		resolved := c.physical[name]
		if resolved.Repository.upstream.Enabled() {
			out = append(out, name)
		}
	}
	return out
}

// DeclaredArches returns the validated architecture set for a physical repo.
func (c *RepositoryCatalog) DeclaredArches(physical string) []string {
	resolved, ok := c.Resolve(physical)
	if !ok {
		return nil
	}
	return resolved.Repository.Arches()
}

func (c *RepositoryCatalog) AllowsNewArch(physical string) bool {
	resolved, ok := c.Resolve(physical)
	return ok && resolved.Repository.allowNewArch
}

// PublishTarget resolves an upload target. The logical name of a tiered repo
// deliberately means staging; an explicit physical tier remains unchanged.
func (c *RepositoryCatalog) PublishTarget(name string) (string, bool) {
	if repository, ok := c.Logical(name); ok {
		if repository.tiered {
			return repository.name + "-" + string(TierStaging), true
		}
		return repository.name, true
	}
	if resolved, ok := c.Resolve(name); ok {
		return resolved.Physical, true
	}
	return "", false
}
