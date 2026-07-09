// Package depend resolves the AUR packages a build needs but the configured
// pacman repos cannot provide. Given the dependencies pacman cannot satisfy, it
// recurses through the AUR, builds a dependency graph keyed by package base, and
// returns the bases in build order (dependencies first). The actual build and
// publish of each base is the caller's job (miko); this package is pure logic
// over the RepoChecker/AURSource seams so it stays testable.
package depend

import (
	"context"
	"fmt"
)

// Pkg is the minimal AUR package metadata the resolver needs.
type Pkg struct {
	Name        string
	PackageBase string
	Version     string
	Provides    []string // raw provides entries, e.g. "cc=1:13.2"
	Deps        []string // depends + makedepends + checkdepends, raw specs
}

// RepoChecker reports which dep specs the build environment cannot satisfy;
// used as a best-effort filter before querying the AUR (specs not in the AUR are treated as repo-provided).
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

// Resolve returns AUR package bases (deps-first) needed for rootDeps the repos cannot satisfy;
// deps not in the AUR are treated as repo-provided. Errors on unsatisfiable version constraint or cycle.
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

// visit does a post-order DFS (base appended after all its deps); a gray node re-encountered is a cycle.
func (r *resolver) visit(p Pkg) error {
	switch r.color[p.PackageBase] {
	case black:
		return nil
	case gray:
		return fmt.Errorf("depend: dependency cycle through %q", p.PackageBase)
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

// resolveSpec finds the AUR package satisfying spec: by name, then by provides.
// Returns nil (no error) when not in the AUR (treated as repo-provided).
func (r *resolver) resolveSpec(spec string) (*Pkg, error) {
	c := Parse(spec)

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
			return nil, fmt.Errorf("depend: AUR %s-%s does not satisfy %q", p.Name, p.Version, spec)
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

// checkProvides verifies a versioned dep against a provider's provides;
// unversioned constraints always pass, mirroring pacman.
func checkProvides(c Constraint, p Pkg) error {
	if c.Op == OpAny {
		return nil
	}
	for _, pv := range p.Provides {
		pc := Parse(pv)
		if pc.Name != c.Name || pc.Op == OpAny {
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
	return fmt.Errorf("depend: %s (provided by %s) does not satisfy %q%s", c.Name, p.PackageBase, c.Op, c.Ver)
}
