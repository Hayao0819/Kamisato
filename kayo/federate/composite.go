// Package federate merges several aurweb backends into one, resolving name
// collisions by priority: the highest-priority source that provides a package
// wins. Local git overlays rank above ayato instances, which rank above the
// upstream AUR (the upstream stays a separate fallback on the aurweb.Server).
package federate

import (
	"cmp"
	"context"
	"log/slog"
	"slices"

	"github.com/Hayao0819/Kamisato/kayo/trust"
	"github.com/Hayao0819/Kamisato/pkg/aurweb"
)

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
	// delegated marks a source whose signed catalog is vouched: while verified
	// reports true, its packages bypass the trust store entirely. verified is the
	// live check, so a failed re-sync falls closed back to gating.
	delegated bool
	verified  func() bool
}

// delegatedVerified reports whether this source's signed catalog currently vouches
// for its packages (delegated and live-verified), so they bypass the trust store.
func (e entry) delegatedVerified() bool {
	return e.delegated && e.verified != nil && e.verified()
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

func (c *Composite) Add(b aurweb.Backend, tier Tier, priority int, source string) {
	c.add(entry{backend: b, tier: tier, priority: priority, source: source})
}

// AddDelegated registers a source whose verified catalog bypasses the trust
// gate. verified is the live verification check; the bypass only holds while it
// returns true, so a failed re-sync fails closed back to ordinary gating.
func (c *Composite) AddDelegated(b aurweb.Backend, tier Tier, priority int, source string, verified func() bool) {
	c.add(entry{backend: b, tier: tier, priority: priority, source: source, delegated: true, verified: verified})
}

func (c *Composite) add(e entry) {
	c.entries = append(c.entries, e)
	slices.SortStableFunc(c.entries, func(a, b entry) int {
		if a.tier != b.tier {
			return cmp.Compare(b.tier, a.tier) // higher tier first
		}
		return cmp.Compare(b.priority, a.priority) // higher priority first
	})
}

// keep applies the trust gate to a package from entry e; a delegated source whose
// attestation currently verifies bypasses the store inside gate.
func (c *Composite) keep(e entry, p aurweb.Pkg) (aurweb.Pkg, bool) {
	return gate(c.store, c.mode, e.source, e.delegatedVerified(), p)
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
			if gp, keep := c.keep(e, p); keep {
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
			if gp, keep := c.keep(e, p); keep {
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
	if len(out) > aurweb.SuggestLimit {
		out = out[:aurweb.SuggestLimit]
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
			if gp, keep := c.keep(e, p); keep {
				out = append(out, gp)
			}
		}
	}
	return out, nil
}

func (c *Composite) SourceURL(ctx context.Context, pkgbase string) (string, bool, error) {
	for _, e := range c.entries {
		u, ok, err := e.backend.SourceURL(ctx, pkgbase)
		if err != nil {
			slog.Error("source url failed", "source", e.source, "error", err)
			continue
		}
		if ok {
			return u, true, nil
		}
	}
	return "", false, nil
}

// Resolve returns the winning record for a pkgname and the trust namespace of
// the source that provided it (highest tier, then priority). Unlike Info it is
// ungated, so a caller can apply its own trust evaluation. delegatedVerified
// reports the keep() bypass: the winning source is delegated and its attestation
// currently verifies, so the caller should treat the package as trusted without
// consulting the trust store.
func (c *Composite) Resolve(ctx context.Context, name string) (pkg aurweb.Pkg, source string, delegatedVerified bool, ok bool) {
	for _, e := range c.entries {
		pkgs, err := e.backend.Info(ctx, []string{name})
		if err != nil {
			continue
		}
		for _, p := range pkgs {
			if p.Name == name {
				return p, e.source, e.delegatedVerified(), true
			}
		}
	}
	return aurweb.Pkg{}, "", false, false
}
