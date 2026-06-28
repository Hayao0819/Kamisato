package federate

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Hayao0819/Kamisato/pkg/aurweb"
	"github.com/Hayao0819/Kamisato/sara/trust"
)

type stub struct {
	pkgs map[string]aurweb.Pkg
	src  map[string]string
}

func (s *stub) Info(_ context.Context, names []string) ([]aurweb.Pkg, error) {
	var out []aurweb.Pkg
	for _, n := range names {
		if p, ok := s.pkgs[n]; ok {
			out = append(out, p)
		}
	}
	return out, nil
}
func (s *stub) Search(_ context.Context, _ aurweb.By, _ string) ([]aurweb.Pkg, error) {
	return s.All(nil)
}
func (s *stub) Suggest(_ context.Context, _ string, _ bool) ([]string, error) { return nil, nil }
func (s *stub) All(_ context.Context) ([]aurweb.Pkg, error) {
	out := make([]aurweb.Pkg, 0, len(s.pkgs))
	for _, p := range s.pkgs {
		out = append(out, p)
	}
	return out, nil
}
func (s *stub) SourceURL(_ context.Context, pkgbase string) (string, bool, error) {
	if u, ok := s.src[pkgbase]; ok {
		return u, true, nil
	}
	return "", false, nil
}

func TestCompositePriority(t *testing.T) {
	high := &stub{
		pkgs: map[string]aurweb.Pkg{"foo": {Name: "foo", PackageBase: "foo", Version: "2"}},
		src:  map[string]string{"foo": "high-url"},
	}
	low := &stub{
		pkgs: map[string]aurweb.Pkg{
			"foo": {Name: "foo", PackageBase: "foo", Version: "1"},
			"bar": {Name: "bar", PackageBase: "bar", Version: "1"},
		},
		src: map[string]string{"foo": "low-url", "bar": "bar-url"},
	}

	c := New()
	c.Add(low, TierAyato, 1, "low")
	c.Add(high, TierAyato, 10, "high")
	ctx := context.Background()

	info, _ := c.Info(ctx, []string{"foo"})
	if len(info) != 1 || info[0].Version != "2" {
		t.Fatalf("higher priority should win: %+v", info)
	}

	if u, ok, _ := c.SourceURL(ctx, "foo"); !ok || u != "high-url" {
		t.Errorf("SourceURL = %q ok=%v, want high-url", u, ok)
	}

	all, _ := c.All(ctx)
	if len(all) != 2 {
		t.Errorf("All should dedupe foo and include bar: %d entries", len(all))
	}
}

func TestGate(t *testing.T) {
	st, _ := trust.Open(filepath.Join(t.TempDir(), "t.json"))
	st.Approve(trust.Approval{Pkgbase: "yay", Source: "aur", Maintainer: "jguer", Commit: "c1"})

	unreviewed := aurweb.Pkg{Name: "new", PackageBase: "new", Maintainer: "someone", Description: "d"}
	takeover := aurweb.Pkg{Name: "yay", PackageBase: "yay", Maintainer: "attacker", Description: "d"}

	// enforce: unreviewed and takeover are both dropped.
	if _, keep := gate(st, "enforce", "aur", unreviewed); keep {
		t.Error("enforce should drop unreviewed package")
	}
	if _, keep := gate(st, "enforce", "aur", takeover); keep {
		t.Error("enforce should drop maintainer-changed package")
	}

	// warn: unreviewed passes unannotated (avoid noise on every AUR package).
	if gp, keep := gate(st, "warn", "aur", unreviewed); !keep || strings.HasPrefix(gp.Description, "[sara") {
		t.Errorf("warn should pass unreviewed unannotated: keep=%v desc=%q", keep, gp.Description)
	}
	// warn: a violated approval (maintainer changed) is annotated.
	if gp, keep := gate(st, "warn", "aur", takeover); !keep || !strings.Contains(gp.Description, "maintainer changed") {
		t.Errorf("warn should annotate takeover: keep=%v desc=%q", keep, gp.Description)
	}
	// overlays are trusted by config regardless of mode.
	if gp, keep := gate(st, "enforce", "overlay", unreviewed); !keep || strings.HasPrefix(gp.Description, "[sara") {
		t.Errorf("overlay should always pass clean: keep=%v desc=%q", keep, gp.Description)
	}
}

func TestDelegatedBypass(t *testing.T) {
	st, _ := trust.Open(filepath.Join(t.TempDir(), "t.json")) // empty: nothing reviewed
	pkgs := map[string]aurweb.Pkg{"new": {Name: "new", PackageBase: "new", Maintainer: "someone"}}

	// enforce mode drops an unreviewed package from an ordinary source...
	plain := New()
	plain.SetGate(st, "enforce")
	plain.Add(&stub{pkgs: pkgs}, TierAyato, 0, "plain")
	if info, _ := plain.Info(context.Background(), []string{"new"}); len(info) != 0 {
		t.Fatalf("enforce should drop unreviewed from a gated source: %+v", info)
	}

	// ...but a delegated source whose attestation verifies bypasses the gate.
	verified := true
	deleg := New()
	deleg.SetGate(st, "enforce")
	deleg.AddDelegated(&stub{pkgs: pkgs}, TierAyato, 0, "deleg", func() bool { return verified })
	if info, _ := deleg.Info(context.Background(), []string{"new"}); len(info) != 1 {
		t.Fatalf("delegated+verified should bypass the gate: %+v", info)
	}

	// When live verification drops (a failed re-sync), it falls closed to gating.
	verified = false
	if info, _ := deleg.Info(context.Background(), []string{"new"}); len(info) != 0 {
		t.Fatalf("delegated but unverified must fall closed to gating: %+v", info)
	}
}

func TestResolve(t *testing.T) {
	high := &stub{pkgs: map[string]aurweb.Pkg{"foo": {Name: "foo", PackageBase: "foo"}}}
	low := &stub{pkgs: map[string]aurweb.Pkg{"foo": {Name: "foo"}, "bar": {Name: "bar", PackageBase: "bar"}}}
	c := New()
	c.Add(low, TierAyato, 1, "low")
	c.Add(high, TierAyato, 10, "high")
	ctx := context.Background()

	if _, src, ok := c.Resolve(ctx, "foo"); !ok || src != "high" {
		t.Errorf("foo resolved from %q ok=%v, want high tier", src, ok)
	}
	if _, src, ok := c.Resolve(ctx, "bar"); !ok || src != "low" {
		t.Errorf("bar resolved from %q, want low", src)
	}
	if _, _, ok := c.Resolve(ctx, "zzz"); ok {
		t.Error("zzz should not resolve")
	}
}
