// Package depsolve resolves the AUR packages a build needs but the configured
// pacman repos cannot provide. Given the dependencies pacman cannot satisfy, it
// recurses through the AUR, builds a dependency graph keyed by package base, and
// returns the bases in build order (dependencies first). The actual build and
// publish of each base is the caller's job (miko); this package is pure logic
// over the RepoChecker/AURSource seams so it stays testable.
package depsolve

import (
	"context"
	"fmt"

	"github.com/Hayao0819/Kamisato/pkg/pacman/dep"
)

// Pkg is the minimal AUR package metadata the resolver needs.
type Pkg struct {
	Name        string
	PackageBase string
	Version     string
	Provides    []string // raw provides entries, e.g. "cc=1:13.2"
	Deps        []string // depends + makedepends + checkdepends, raw specs
}

// RepoChecker reports which of the given dep specs the build environment cannot
// already provide, so only those need resolving against the AUR. It is a
// best-effort filter (an optimisation): a spec it lets through but which is not in
// the AUR is treated as repo-provided and skipped by the resolver.
type RepoChecker interface {
	Unsatisfied(deps []string) ([]string, error)
}

// AURSource looks packages up in the AUR.
type AURSource interface {
	// Info returns metadata for any of names that exist in the AUR (by name).
	Info(ctx context.Context, names []string) ([]Pkg, error)
	// ProvidedBy returns an AUR package whose provides covers name, or nil.
	ProvidedBy(ctx context.Context, name string) (*Pkg, error)
}

// Resolve returns the AUR package bases to build, dependencies first, so that
// rootDeps the repos cannot satisfy become buildable. Dependencies not found in
// the AUR are treated as repo-provided (the build environment's pacman installs
// them) and skipped; it errors on an unsatisfiable version constraint or a cycle.
func Resolve(ctx context.Context, rootDeps []string, repo RepoChecker, aur AURSource) ([]Pkg, error) {
	missing, err := repo.Unsatisfied(rootDeps)
	if err != nil {
		return nil, err
	}
	r := &resolver{ctx: ctx, repo: repo, aur: aur, color: map[string]int{}}
	for _, spec := range missing {
		p, err := r.resolveSpec(spec)
		if err != nil {
			return nil, err
		}
		if p == nil {
			continue
		}
		if err := r.visit(*p); err != nil {
			return nil, err
		}
	}
	return r.order, nil
}

const (
	white = iota // unseen
	gray         // on the current DFS path (cycle marker)
	black        // finished
)

type resolver struct {
	ctx   context.Context
	repo  RepoChecker
	aur   AURSource
	color map[string]int // pkgbase -> color
	order []Pkg
}

// visit does a post-order DFS so a base is appended only after its dependencies,
// giving build order. A gray node re-encountered is a cycle.
func (r *resolver) visit(p Pkg) error {
	switch r.color[p.PackageBase] {
	case black:
		return nil
	case gray:
		return fmt.Errorf("depsolve: dependency cycle through %q", p.PackageBase)
	}
	r.color[p.PackageBase] = gray

	missing, err := r.repo.Unsatisfied(p.Deps)
	if err != nil {
		return err
	}
	for _, spec := range missing {
		d, err := r.resolveSpec(spec)
		if err != nil {
			return fmt.Errorf("resolving dependency of %q: %w", p.PackageBase, err)
		}
		if d == nil {
			continue
		}
		if err := r.visit(*d); err != nil {
			return err
		}
	}

	r.color[p.PackageBase] = black
	r.order = append(r.order, p)
	return nil
}

// resolveSpec finds the AUR package satisfying one dep spec: by name first, then
// by provides for a virtual dependency, verifying the version constraint. It
// returns nil (no error) when the spec is not in the AUR, meaning it is provided
// by the build environment's repos and needs no build.
func (r *resolver) resolveSpec(spec string) (*Pkg, error) {
	c := dep.Parse(spec)

	pkgs, err := r.aur.Info(r.ctx, []string{c.Name})
	if err != nil {
		return nil, err
	}
	for _, p := range pkgs {
		if p.Name != c.Name {
			continue
		}
		ok, err := c.Satisfies(p.Version)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, fmt.Errorf("depsolve: AUR %s-%s does not satisfy %q", p.Name, p.Version, spec)
		}
		return &p, nil
	}

	p, err := r.aur.ProvidedBy(r.ctx, c.Name)
	if err != nil {
		return nil, err
	}
	if p != nil {
		if err := checkProvides(c, *p); err != nil {
			return nil, err
		}
		return p, nil
	}

	return nil, nil
}

// checkProvides verifies a versioned dependency against a provider's provides
// entries. An unversioned constraint always passes; a versioned one requires a
// matching versioned provides ("name=ver"), mirroring pacman.
func checkProvides(c dep.Constraint, p Pkg) error {
	if c.Op == dep.OpAny {
		return nil
	}
	for _, pv := range p.Provides {
		pc := dep.Parse(pv)
		if pc.Name != c.Name || pc.Op == dep.OpAny {
			continue
		}
		ok, err := c.Satisfies(pc.Ver)
		if err != nil {
			return err
		}
		if ok {
			return nil
		}
	}
	return fmt.Errorf("depsolve: %s (provided by %s) does not satisfy %q%s", c.Name, p.PackageBase, c.Op, c.Ver)
}
