package buildset

import (
	"context"
	"fmt"
	"slices"
	"strings"

	pacmanpkg "github.com/Hayao0819/Kamisato/internal/pacman"
	"github.com/Hayao0819/Kamisato/pkg/aurweb"
)

type planner struct {
	ctx                 context.Context
	application         *Application
	arch                string
	sourcesDir          string
	pacmanConf          string
	localRepositoryDirs []string
	repositories        []repositorySnapshot
	sources             []*sourceRecord
	byBase              map[string]*sourceRecord
	aurInfo             map[string][]aurweb.Pkg
	aurSearch           map[string][]aurweb.Pkg
}

func newPlanner(
	ctx context.Context,
	application *Application,
	arch, sourcesDir, pacmanConf string,
	localRepositoryDirs []string,
	repositories []repositorySnapshot,
	local []*sourceRecord,
) *planner {
	byBase := make(map[string]*sourceRecord, len(local))
	for _, source := range local {
		byBase[source.metadata.Base()] = source
	}
	return &planner{
		ctx:                 ctx,
		application:         application,
		arch:                arch,
		sourcesDir:          sourcesDir,
		pacmanConf:          pacmanConf,
		localRepositoryDirs: append([]string(nil), localRepositoryDirs...),
		repositories:        repositories,
		sources:             append([]*sourceRecord(nil), local...),
		byBase:              byBase,
		aurInfo:             map[string][]aurweb.Pkg{},
		aurSearch:           map[string][]aurweb.Pkg{},
	}
}

func (p *planner) build(requested []string) (*Plan, error) {
	install := map[string]struct{}{}
	for _, source := range p.sources {
		for name := range source.explicit {
			install[name] = struct{}{}
		}
	}
	for _, name := range requested {
		local := p.exactSourceCandidates(name, SourceLocal)
		if len(local) > 1 {
			return nil, fmt.Errorf("requested package %q has multiple local sources: %s", name, strings.Join(sourceBases(local), ", "))
		}
		if len(local) == 1 {
			local[0].explicit[name] = struct{}{}
			install[name] = struct{}{}
			continue
		}
		entry, err := p.exactAURPackage(name)
		if err != nil {
			return nil, err
		}
		if entry == nil {
			return nil, fmt.Errorf("requested package %q was not found in local sources or AUR", name)
		}
		source, err := p.addAURSource(entry.PackageBase)
		if err != nil {
			return nil, fmt.Errorf("resolve requested package %q: %w", name, err)
		}
		if !slices.Contains(source.metadata.OutputNames(p.arch), name) {
			return nil, fmt.Errorf("AUR pkgbase %q does not produce requested package %q for %s", source.metadata.Base(), name, p.arch)
		}
		source.explicit[name] = struct{}{}
		install[name] = struct{}{}
	}

	for {
		added := false
		for _, source := range p.sources {
			resolved := make([]Dependency, 0, len(source.metadata.BuildDepends(p.arch)))
			for _, spec := range source.metadata.BuildDepends(p.arch) {
				dependency, sourceAdded, err := p.resolveDependency(source, spec)
				if err != nil {
					return nil, err
				}
				if sourceAdded {
					added = true
					break
				}
				resolved = append(resolved, dependency)
			}
			if added {
				break
			}
			source.dependencies = resolved
		}
		if !added {
			break
		}
	}

	nodes := make([]string, 0, len(p.sources))
	edges := make(map[string][]string, len(p.sources))
	for _, source := range p.sources {
		base := source.metadata.Base()
		nodes = append(nodes, base)
		for _, dependency := range source.dependencies {
			if dependency.ProviderPkgbase != "" && dependency.ProviderPkgbase != base {
				edges[base] = append(edges[base], dependency.ProviderPkgbase)
			}
		}
	}
	order, err := pacmanpkg.NewDepGraph(nodes, edges).BuildOrder()
	if err != nil {
		return nil, err
	}
	builds := make([]PlannedBuild, 0, len(order))
	for _, base := range order {
		source := p.byBase[base]
		explicit := mapKeys(source.explicit)
		builds = append(builds, PlannedBuild{
			Pkgbase:      base,
			Version:      source.metadata.Version(),
			Source:       source.ref,
			Packages:     source.metadata.OutputNames(p.arch),
			Explicit:     explicit,
			Dependencies: append([]Dependency(nil), source.dependencies...),
		})
	}
	return &Plan{
		SchemaVersion: SchemaVersion,
		Arch:          p.arch,
		Install:       mapKeys(install),
		BuildOrder:    order,
		Builds:        builds,
	}, nil
}

func (p *planner) resolveDependency(owner *sourceRecord, spec string) (Dependency, bool, error) {
	if dependency, ok, err := p.resolveFromSources(spec, SourceLocal, "local"); ok || err != nil {
		return dependency, false, wrapDependencyError(owner, spec, err)
	}
	if dependency, ok, err := p.resolveFromSources(spec, SourceAUR, "built"); ok || err != nil {
		return dependency, false, wrapDependencyError(owner, spec, err)
	}
	for _, repository := range p.repositories {
		candidates := repositoryCandidates(repository, spec, p.arch)
		if len(candidates) == 0 {
			continue
		}
		if len(candidates) > 1 {
			return Dependency{}, false, fmt.Errorf("pkgbase %q dependency %q has multiple providers in repository %q: %s", owner.metadata.Base(), spec, repository.name, strings.Join(candidates, ", "))
		}
		return Dependency{Constraint: spec, ProviderType: "repository", Provider: candidates[0], Repository: repository.name}, false, nil
	}

	candidate, err := p.resolveAURCandidate(spec)
	if err != nil {
		return Dependency{}, false, wrapDependencyError(owner, spec, err)
	}
	if candidate == nil {
		return Dependency{}, false, fmt.Errorf("pkgbase %q dependency %q has no provider in local sources, configured repositories, or AUR", owner.metadata.Base(), spec)
	}
	if _, exists := p.byBase[candidate.PackageBase]; exists {
		return Dependency{}, false, fmt.Errorf("pkgbase %q dependency %q is not satisfied by already selected AUR pkgbase %q", owner.metadata.Base(), spec, candidate.PackageBase)
	}
	if _, err := p.addAURSource(candidate.PackageBase); err != nil {
		return Dependency{}, false, wrapDependencyError(owner, spec, err)
	}
	return Dependency{}, true, nil
}

func (p *planner) resolveFromSources(spec string, kind SourceType, providerType string) (Dependency, bool, error) {
	constraint := pacmanpkg.Parse(spec)
	var candidates []*sourceRecord
	for _, source := range p.sources {
		if source.ref.Type != kind || !sourceSatisfies(source, constraint, p.arch) {
			continue
		}
		candidates = append(candidates, source)
	}
	if len(candidates) == 0 {
		return Dependency{}, false, nil
	}
	if len(candidates) > 1 {
		return Dependency{}, false, fmt.Errorf("multiple %s providers: %s", providerType, strings.Join(sourceBases(candidates), ", "))
	}
	return Dependency{
		Constraint:      spec,
		ProviderType:    providerType,
		Provider:        candidates[0].metadata.Base(),
		ProviderPkgbase: candidates[0].metadata.Base(),
	}, true, nil
}

func sourceSatisfies(source *sourceRecord, constraint pacmanpkg.Constraint, arch string) bool {
	if slices.Contains(source.metadata.OutputNames(arch), constraint.Name) {
		matches, _ := constraint.Satisfies(source.metadata.Version())
		if matches {
			return true
		}
	}
	for _, provided := range source.metadata.OutputProvides(arch) {
		if providedSatisfies(constraint, provided) {
			return true
		}
	}
	return false
}

func (p *planner) exactSourceCandidates(name string, kind SourceType) []*sourceRecord {
	var candidates []*sourceRecord
	for _, source := range p.sources {
		if source.ref.Type == kind && slices.Contains(source.metadata.OutputNames(p.arch), name) {
			candidates = append(candidates, source)
		}
	}
	return candidates
}

func (p *planner) exactAURPackage(name string) (*aurweb.Pkg, error) {
	entries, err := p.aurInfoFor(name)
	if err != nil {
		return nil, err
	}
	var exact []aurweb.Pkg
	for _, entry := range entries {
		if entry.Name == name {
			exact = append(exact, entry)
		}
	}
	if len(exact) == 0 {
		return nil, nil
	}
	if len(exact) > 1 {
		return nil, fmt.Errorf("AUR returned multiple exact matches for %q", name)
	}
	return &exact[0], nil
}

func (p *planner) resolveAURCandidate(spec string) (*aurweb.Pkg, error) {
	constraint := pacmanpkg.Parse(spec)
	if exact, err := p.exactAURPackage(constraint.Name); err != nil {
		return nil, err
	} else if exact != nil {
		matches, err := constraint.Satisfies(exact.Version)
		if err != nil {
			return nil, err
		}
		if matches {
			return exact, nil
		}
	}

	entries, err := p.aurProvidersFor(constraint.Name)
	if err != nil {
		return nil, err
	}
	byBase := map[string]aurweb.Pkg{}
	for _, entry := range entries {
		for _, provided := range entry.Provides {
			if providedSatisfies(constraint, provided) {
				byBase[entry.PackageBase] = entry
				break
			}
		}
	}
	bases := make([]string, 0, len(byBase))
	for base := range byBase {
		bases = append(bases, base)
	}
	slices.Sort(bases)
	if len(bases) == 0 {
		return nil, nil
	}
	if len(bases) > 1 {
		return nil, fmt.Errorf("AUR has multiple providers: %s", strings.Join(bases, ", "))
	}
	entry := byBase[bases[0]]
	return &entry, nil
}

func (p *planner) aurInfoFor(name string) ([]aurweb.Pkg, error) {
	if cached, exists := p.aurInfo[name]; exists {
		return cached, nil
	}
	entries, err := p.application.aur.Info(p.ctx, []string{name})
	if err != nil {
		return nil, fmt.Errorf("query AUR package %q: %w", name, err)
	}
	p.aurInfo[name] = entries
	return entries, nil
}

func (p *planner) aurProvidersFor(name string) ([]aurweb.Pkg, error) {
	if cached, exists := p.aurSearch[name]; exists {
		return cached, nil
	}
	results, err := p.application.aur.Search(p.ctx, aurweb.ByProvides, name)
	if err != nil {
		return nil, fmt.Errorf("search AUR providers for %q: %w", name, err)
	}
	requested := make([]string, 0, len(results))
	for _, result := range results {
		requested = append(requested, result.Name)
	}
	slices.Sort(requested)
	requested = slices.Compact(requested)
	var entries []aurweb.Pkg
	if len(requested) > 0 {
		entries, err = p.application.aur.Info(p.ctx, requested)
		if err != nil {
			return nil, fmt.Errorf("query AUR provider metadata for %q: %w", name, err)
		}
	}
	p.aurSearch[name] = entries
	return entries, nil
}

func (p *planner) addAURSource(pkgbase string) (*sourceRecord, error) {
	if existing, exists := p.byBase[pkgbase]; exists {
		if existing.ref.Type == SourceLocal {
			return nil, fmt.Errorf("local pkgbase %q shadows the requested AUR source", pkgbase)
		}
		return existing, nil
	}
	source, err := p.application.fetchAURSource(
		p.ctx, pkgbase, p.sourcesDir, p.arch, p.pacmanConf, p.localRepositoryDirs,
	)
	if err != nil {
		return nil, err
	}
	p.sources = append(p.sources, source)
	p.byBase[pkgbase] = source
	return source, nil
}

func sourceBases(sources []*sourceRecord) []string {
	bases := make([]string, 0, len(sources))
	for _, source := range sources {
		bases = append(bases, source.metadata.Base())
	}
	slices.Sort(bases)
	return slices.Compact(bases)
}

func mapKeys(values map[string]struct{}) []string {
	keys := make([]string, 0, len(values))
	for value := range values {
		keys = append(keys, value)
	}
	slices.Sort(keys)
	return keys
}

func wrapDependencyError(owner *sourceRecord, spec string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("pkgbase %q dependency %q: %w", owner.metadata.Base(), spec, err)
}
