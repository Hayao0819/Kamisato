package domain

import (
	"fmt"
	"slices"
)

// ResolvedRepository identifies the logical policy behind one concrete pacman
// repository name and, for tiered repositories, its promotion tier.
type ResolvedRepository struct {
	Repository *Repository
	Physical   string
	Tier       Tier
}

// RepositoryCatalog is the single index for logical policies and physical
// repository names. It is immutable after construction.
type RepositoryCatalog struct {
	logicalOrder  []string
	physicalOrder []string
	logical       map[string]*Repository
	physical      map[string]ResolvedRepository
}

// NewRepositoryCatalog validates the complete topology, including physical-name
// collisions such as logical "core-staging" beside tiered "core".
func NewRepositoryCatalog(
	defaultArches []string,
	specs []RepositorySpec,
) (*RepositoryCatalog, error) {
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
		if err := catalog.add(repository); err != nil {
			return nil, err
		}
	}
	return catalog, nil
}

func (c *RepositoryCatalog) add(repository *Repository) error {
	if _, duplicate := c.logical[repository.name]; duplicate {
		return fmt.Errorf(
			"logical repository name %q is duplicated",
			repository.name,
		)
	}
	c.logical[repository.name] = repository
	c.logicalOrder = append(c.logicalOrder, repository.name)
	for _, tier := range repository.tiers() {
		if err := c.addPhysical(repository, tier); err != nil {
			return err
		}
	}
	return nil
}

func (c *RepositoryCatalog) addPhysical(
	repository *Repository,
	tier Tier,
) error {
	physical, err := repository.PhysicalName(tier)
	if err != nil {
		return err
	}
	if previous, collision := c.physical[physical]; collision {
		return fmt.Errorf(
			"physical repository name %q collides between logical repositories %q and %q",
			physical,
			previous.Repository.name,
			repository.name,
		)
	}
	c.physical[physical] = ResolvedRepository{
		Repository: repository,
		Physical:   physical,
		Tier:       tier,
	}
	c.physicalOrder = append(c.physicalOrder, physical)
	return nil
}

func (c *RepositoryCatalog) Logical(name string) (*Repository, bool) {
	if c == nil {
		return nil, false
	}
	repository, ok := c.logical[name]
	return repository, ok
}

func (c *RepositoryCatalog) Resolve(
	name string,
) (ResolvedRepository, bool) {
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

func (c *RepositoryCatalog) UpstreamPhysicalNames() []string {
	if c == nil {
		return nil
	}
	names := make([]string, 0, len(c.physicalOrder))
	for _, name := range c.physicalOrder {
		if c.physical[name].Repository.upstream.Enabled() {
			names = append(names, name)
		}
	}
	return names
}

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

// PublishTarget maps a tiered logical name to staging. Explicit physical names
// retain their addressed tier.
func (c *RepositoryCatalog) PublishTarget(name string) (string, bool) {
	if repository, ok := c.Logical(name); ok {
		if repository.tiered {
			physical, _ := repository.PhysicalName(TierStaging)
			return physical, true
		}
		return repository.name, true
	}
	if resolved, ok := c.Resolve(name); ok {
		return resolved.Physical, true
	}
	return "", false
}
