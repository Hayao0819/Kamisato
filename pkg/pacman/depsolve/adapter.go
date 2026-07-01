package depsolve

import (
	"context"

	"github.com/Hayao0819/Kamisato/pkg/aurweb"
	"github.com/Hayao0819/Kamisato/pkg/pacman/alpm"
)

// NewRepoChecker returns a RepoChecker backed by `pacman -T` against the running
// system's locally installed packages. It is a best-effort pre-filter: specs it
// lets through that are not in the AUR are treated as repo-provided by the
// resolver, so a dep already available in the build environment's sync repos does
// not need to be installed on this host for resolution to succeed.
func NewRepoChecker() RepoChecker { return alpmRepoChecker{} }

type alpmRepoChecker struct{}

func (alpmRepoChecker) Unsatisfied(deps []string) ([]string, error) {
	return alpm.Deptest(deps)
}

// NewAURSource adapts an aurweb upstream client to the AURSource seam.
func NewAURSource(up *aurweb.AURUpstream) AURSource { return aurSource{up: up} }

type aurSource struct {
	up *aurweb.AURUpstream
}

func (a aurSource) Info(ctx context.Context, names []string) ([]Pkg, error) {
	ps, err := a.up.Info(ctx, names)
	if err != nil {
		return nil, err
	}
	out := make([]Pkg, 0, len(ps))
	for _, p := range ps {
		out = append(out, fromAUR(p))
	}
	return out, nil
}

func (a aurSource) ProvidedBy(ctx context.Context, name string) (*Pkg, error) {
	ps, err := a.up.Search(ctx, aurweb.ByProvides, name)
	if err != nil {
		return nil, err
	}
	if len(ps) == 0 {
		return nil, nil
	}
	// aurweb search records omit the relation arrays (provides/depends), so fetch
	// the full info record: the resolver needs the provider's version-qualified
	// provides to satisfy a versioned constraint and its own deps to recurse.
	infos, err := a.up.Info(ctx, []string{ps[0].Name})
	if err != nil {
		return nil, err
	}
	if len(infos) == 0 {
		p := fromAUR(ps[0])
		return &p, nil
	}
	p := fromAUR(infos[0])
	return &p, nil
}

func fromAUR(p aurweb.Pkg) Pkg {
	deps := make([]string, 0, len(p.Depends)+len(p.MakeDepends)+len(p.CheckDepends))
	deps = append(deps, p.Depends...)
	deps = append(deps, p.MakeDepends...)
	deps = append(deps, p.CheckDepends...)
	return Pkg{
		Name:        p.Name,
		PackageBase: p.PackageBase,
		Version:     p.Version,
		Provides:    p.Provides,
		Deps:        deps,
	}
}
