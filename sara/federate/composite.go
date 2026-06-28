// Package federate merges several aurweb backends into one, resolving name
// collisions by priority: the highest-priority source that provides a package
// wins. Local git overlays rank above ayato instances, which rank above the
// upstream AUR (the upstream stays a separate fallback on the aurweb.Server).
package federate

import (
	"context"
	"log/slog"
	"sort"

	"github.com/Hayao0819/Kamisato/pkg/aurweb"
	"github.com/Hayao0819/Kamisato/sara/trust"
)

// Syncer is a source that can refresh itself.
type Syncer interface {
	Sync(ctx context.Context) error
}

// Tier ranks sources by trust. A higher tier always wins a name collision,
// regardless of configured priority, so a low-trust source can never shadow a
// higher-trust one. priority only breaks ties within a tier.
type Tier int

const (
	TierAyato   Tier = iota // a federated, external instance
	TierOverlay             // the user's own git overlays (most trusted)
)

type entry struct {
	backend  aurweb.Backend
	tier     Tier
	priority int
	source   string // trust namespace: "overlay" | ayato name
}

// Composite is an aurweb.Backend over an ordered set of sources, applying the
// trust gate as results are merged.
type Composite struct {
	entries []entry
	store   *trust.Store
	mode    string
}

func New() *Composite { return &Composite{} }

// SetGate enables trust gating with the given store and mode ("warn"|"enforce").
func (c *Composite) SetGate(store *trust.Store, mode string) {
	c.store, c.mode = store, mode
}

// Add registers a source in a trust tier under a namespace. A higher tier
// always wins a collision; priority only breaks ties within a tier.
func (c *Composite) Add(b aurweb.Backend, tier Tier, priority int, source string) {
	c.entries = append(c.entries, entry{backend: b, tier: tier, priority: priority, source: source})
	sort.SliceStable(c.entries, func(i, j int) bool {
		if c.entries[i].tier != c.entries[j].tier {
			return c.entries[i].tier > c.entries[j].tier
		}
		return c.entries[i].priority > c.entries[j].priority
	})
}

// Sync refreshes every source that supports it, logging but not failing on a
// single source's error.
func (c *Composite) Sync(ctx context.Context) error {
	for _, e := range c.entries {
		if s, ok := e.backend.(Syncer); ok {
			if err := s.Sync(ctx); err != nil {
				slog.Error("source sync failed", "error", err)
			}
		}
	}
	return nil
}

func (c *Composite) Info(ctx context.Context, names []string) ([]aurweb.Pkg, error) {
	seen := map[string]bool{}
	var out []aurweb.Pkg
	for _, e := range c.entries {
		pkgs, err := e.backend.Info(ctx, names)
		if err != nil {
			slog.Error("source info failed", "error", err)
			continue
		}
		for _, p := range pkgs {
			if seen[p.Name] {
				continue
			}
			seen[p.Name] = true
			if gp, keep := gate(c.store, c.mode, e.source, p); keep {
				out = append(out, gp)
			}
		}
	}
	return out, nil
}

func (c *Composite) Search(ctx context.Context, by aurweb.By, arg string) ([]aurweb.Pkg, error) {
	seen := map[string]bool{}
	var out []aurweb.Pkg
	for _, e := range c.entries {
		pkgs, err := e.backend.Search(ctx, by, arg)
		if err != nil {
			slog.Error("source search failed", "error", err)
			continue
		}
		for _, p := range pkgs {
			if seen[p.Name] {
				continue
			}
			seen[p.Name] = true
			if gp, keep := gate(c.store, c.mode, e.source, p); keep {
				out = append(out, gp)
			}
		}
	}
	return out, nil
}

func (c *Composite) Suggest(ctx context.Context, arg string, pkgbase bool) ([]string, error) {
	seen := map[string]bool{}
	var out []string
	for _, e := range c.entries {
		names, err := e.backend.Suggest(ctx, arg, pkgbase)
		if err != nil {
			continue
		}
		for _, n := range names {
			if !seen[n] {
				seen[n] = true
				out = append(out, n)
			}
		}
	}
	if len(out) > 20 {
		out = out[:20]
	}
	return out, nil
}

func (c *Composite) All(ctx context.Context) ([]aurweb.Pkg, error) {
	seen := map[string]bool{}
	var out []aurweb.Pkg
	for _, e := range c.entries {
		pkgs, err := e.backend.All(ctx)
		if err != nil {
			continue
		}
		for _, p := range pkgs {
			if seen[p.Name] {
				continue
			}
			seen[p.Name] = true
			if gp, keep := gate(c.store, c.mode, e.source, p); keep {
				out = append(out, gp)
			}
		}
	}
	return out, nil
}

func (c *Composite) SourceURL(ctx context.Context, pkgbase string) (string, bool, error) {
	for _, e := range c.entries {
		if u, ok, err := e.backend.SourceURL(ctx, pkgbase); err == nil && ok {
			return u, true, nil
		}
	}
	return "", false, nil
}

// Resolve returns the winning record for a pkgname and the trust namespace of
// the source that provided it (highest tier, then priority). Unlike Info it is
// ungated, so a caller can apply its own trust evaluation.
func (c *Composite) Resolve(ctx context.Context, name string) (aurweb.Pkg, string, bool) {
	for _, e := range c.entries {
		pkgs, err := e.backend.Info(ctx, []string{name})
		if err != nil {
			continue
		}
		for _, p := range pkgs {
			if p.Name == name {
				return p, e.source, true
			}
		}
	}
	return aurweb.Pkg{}, "", false
}
