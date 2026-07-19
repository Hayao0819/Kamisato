package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/Hayao0819/Kamisato/miko/domain"
	"github.com/Hayao0819/Kamisato/pkg/atomicfile"
	"github.com/Hayao0819/Kamisato/pkg/httpx"
	"github.com/Hayao0819/Kamisato/pkg/pacman/depend"
	ppkg "github.com/Hayao0819/Kamisato/pkg/pacman/pkg"
	"github.com/Hayao0819/Kamisato/pkg/pacman/repo"
)

// sonameStore records the sonames a package provided at its last successful
// build so the next build can detect a bump. The service depends on this seam so
// a fake can stand in for tests; fileSonameStore is the production one.
type sonameStore interface {
	load(pkgbase string) ([]string, error)
	save(pkgbase string, sonames []string) error
}

var _ sonameStore = (*fileSonameStore)(nil)

// fileSonameStore stores one JSON array per pkgbase under <dataDir>/sonames.
type fileSonameStore struct{ dir string }

func newFileSonameStore(dataDir string) (*fileSonameStore, error) {
	dir := filepath.Join(dataDir, "sonames")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, err
	}
	return &fileSonameStore{dir: dir}, nil
}

// path guards against a pkgbase (read from an untrusted package's .PKGINFO) that
// would escape the store dir.
func (s *fileSonameStore) path(pkgbase string) (string, error) {
	if pkgbase == "" || pkgbase != filepath.Base(pkgbase) || strings.ContainsAny(pkgbase, `/\`) {
		return "", fmt.Errorf("unsafe pkgbase %q", pkgbase)
	}
	return filepath.Join(s.dir, pkgbase+".json"), nil
}

func (s *fileSonameStore) load(pkgbase string) ([]string, error) {
	p, err := s.path(pkgbase)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(p)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []string
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *fileSonameStore) save(pkgbase string, sonames []string) error {
	p, err := s.path(pkgbase)
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(sonames, "", "  ")
	if err != nil {
		return err
	}
	return atomicfile.WriteFile(p, data, 0o600)
}

// maybeRebuildOnSonameBump compares the built package's sonames against the last
// recorded set and, on a bump, rebuilds the reverse-dependency chain. It is
// best-effort — errors are logged and panics recovered — so it can never fail
// the primary build, whose status is already final by the time this runs.
func (s *Service) maybeRebuildOnSonameBump(ctx context.Context, job *domain.BuildJob, packages []string) {
	if !s.cfg.SonameRebuild {
		return
	}
	if s.sonames == nil {
		slog.Warn("soname_rebuild is on but no data_dir is set to persist sonames; skipping")
		return
	}
	defer func() {
		if r := recover(); r != nil {
			slog.Error("soname bump detection panicked; primary build unaffected", "recover", r)
		}
	}()

	grouped, err := sonamesByPkgbase(packages)
	if err != nil {
		slog.Warn("could not read sonames from built packages", "err", err)
		return
	}
	for base, current := range grouped {
		prev, err := s.sonames.load(base)
		if err != nil {
			slog.Warn("could not load prior sonames", "pkgbase", base, "err", err)
			continue
		}
		if err := s.sonames.save(base, current); err != nil {
			slog.Warn("could not persist sonames", "pkgbase", base, "err", err)
		}
		if len(prev) == 0 {
			continue // first build: record a baseline, nothing to compare against
		}
		bumps := ppkg.DetectBumps(prev, current)
		if len(bumps) == 0 {
			continue
		}
		slog.Info("soname bump detected", "pkgbase", base, "bumps", bumps)
		s.triggerReverseDepRebuilds(ctx, job, base)
	}
}

// triggerReverseDepRebuilds enqueues the bumped package's reverse-dependency
// chain in dependency order, each tagged ReasonSonameRebuild.
func (s *Service) triggerReverseDepRebuilds(ctx context.Context, job *domain.BuildJob, bumped string) {
	if s.cfg.Ayato.URL == "" || job.Repo == "" || job.Arch == "" {
		slog.Warn("cannot resolve reverse deps without a published repo; skipping soname rebuilds", "pkgbase", bumped)
		return
	}
	rr, err := s.fetchRepoDB(ctx, job.Repo, job.Arch)
	if err != nil {
		slog.Warn("could not fetch repo db for reverse deps", "pkgbase", bumped, "err", err)
		return
	}
	g := repoDepGraph(rr)
	enq := &sonameRebuildEnqueuer{s: s, repo: job.Repo, arch: job.Arch}
	chain, err := enqueueRebuildChain(g, bumped, enq)
	if err != nil {
		slog.Warn("could not enqueue soname rebuild chain", "pkgbase", bumped, "err", err)
		return
	}
	if len(chain) == 0 {
		slog.Info("no reverse dependencies to rebuild for soname bump", "pkgbase", bumped)
		return
	}
	slog.Info("triggered soname reverse-dep rebuilds", "pkgbase", bumped, "count", len(chain), "order", chain)
}

// rebuildEnqueuer starts a rebuild of one pkgbase. The production enqueuer tags
// the job ReasonSonameRebuild; tests use a fake to assert the chain and order.
type rebuildEnqueuer interface {
	enqueueRebuild(pkgbase string) error
}

type sonameRebuildEnqueuer struct {
	s    *Service
	repo string
	arch string
}

func (e *sonameRebuildEnqueuer) enqueueRebuild(pkgbase string) error {
	git := strings.TrimRight(e.s.cfg.AURGitBase, "/") + "/" + pkgbase + ".git"
	req := &domain.BuildRequest{
		Repo: e.repo,
		Arch: e.arch,
		Git:  &domain.GitSource{URL: git},
	}
	id, err := e.s.submitWithReason(req, domain.ReasonSonameRebuild)
	if err != nil {
		return err
	}
	slog.Info("submitted soname rebuild", "id", id, "pkgbase", pkgbase, "repo", e.repo, "arch", e.arch)
	return nil
}

// enqueueRebuildChain enqueues the transitive reverse-dependency chain of bumped
// in dependency order (a dependency rebuilds before its dependents), so a chain
// of rebuilds resolves against freshly-built links.
func enqueueRebuildChain(g *depend.DepGraph, bumped string, enq rebuildEnqueuer) ([]string, error) {
	chain, err := rebuildChain(g, bumped)
	if err != nil {
		return nil, err
	}
	for _, pb := range chain {
		if err := enq.enqueueRebuild(pb); err != nil {
			return chain, err
		}
	}
	return chain, nil
}

// rebuildChain returns the transitive dependents of bumped, ordered by the
// graph's topological build order so dependencies precede the packages that
// depend on them. bumped itself is excluded — it was just built.
func rebuildChain(g *depend.DepGraph, bumped string) ([]string, error) {
	affected := map[string]bool{}
	queue := slices.Clone(g.Dependents(bumped))
	for _, d := range queue {
		affected[d] = true
	}
	for len(queue) > 0 {
		n := queue[0]
		queue = queue[1:]
		for _, d := range g.Dependents(n) {
			if d != bumped && !affected[d] {
				affected[d] = true
				queue = append(queue, d)
			}
		}
	}
	if len(affected) == 0 {
		return nil, nil
	}
	order, err := g.BuildOrder()
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(affected))
	for _, n := range order {
		if affected[n] {
			out = append(out, n)
		}
	}
	return out, nil
}

// repoDepGraph builds a pkgbase dependency graph from a published repo: each
// package's %DEPENDS% (including soname deps like "libfoo.so=1-64") is resolved
// to the pkgbase that provides it (by pkgname or a %PROVIDES% entry), so the
// reverse map answers "who links against this package".
func repoDepGraph(rr *repo.RemoteRepo) *depend.DepGraph {
	provider := map[string]string{}
	bases := map[string]struct{}{}
	baseOf := func(p *ppkgBinary) string {
		if b := p.Base(); b != "" {
			return b
		}
		return p.Name()
	}
	for _, p := range rr.Pkgs {
		base := baseOf(p)
		bases[base] = struct{}{}
		provider[depName(p.Name())] = base
		for _, pr := range p.PKGINFO().Provides {
			provider[depName(pr)] = base
		}
	}
	deps := map[string][]string{}
	for _, p := range rr.Pkgs {
		base := baseOf(p)
		for _, d := range p.PKGINFO().Depend {
			if prov, ok := provider[depName(d)]; ok && prov != base {
				deps[base] = append(deps[base], prov)
			}
		}
	}
	nodes := make([]string, 0, len(bases))
	for b := range bases {
		nodes = append(nodes, b)
	}
	return depend.NewDepGraph(nodes, deps)
}

// ppkgBinary is the concrete binary-package type; aliased so repoDepGraph reads
// cleanly without importing the name into the whole file.
type ppkgBinary = ppkg.BinaryPackage

// depName strips a dependency/provides version constraint, so "libfoo.so=1-64"
// and "bar>=2.0" reduce to the bare name used to match a provider.
func depName(s string) string {
	if i := strings.IndexAny(s, "<>="); i >= 0 {
		return s[:i]
	}
	return s
}

// sonamesByPkgbase reads each built package's pkgbase and the sonames it ships,
// merging the sonames of every subpackage under one pkgbase.
func sonamesByPkgbase(packages []string) (map[string][]string, error) {
	grouped := map[string]map[string]struct{}{}
	for _, p := range packages {
		base, err := pkgbaseOf(p)
		if err != nil {
			return nil, err
		}
		sonames, err := ppkg.SonamesOf(p)
		if err != nil {
			return nil, err
		}
		set := grouped[base]
		if set == nil {
			set = map[string]struct{}{}
			grouped[base] = set
		}
		for _, sn := range sonames {
			set[sn] = struct{}{}
		}
	}
	out := make(map[string][]string, len(grouped))
	for base, set := range grouped {
		list := make([]string, 0, len(set))
		for sn := range set {
			list = append(list, sn)
		}
		slices.Sort(list)
		out[base] = list
	}
	return out, nil
}

func pkgbaseOf(pkgFile string) (string, error) {
	f, err := os.Open(pkgFile)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()
	bp, err := ppkg.ReadBinaryPackage(pkgFile, f)
	if err != nil {
		return "", err
	}
	if b := bp.Base(); b != "" {
		return b, nil
	}
	return bp.Name(), nil
}

func (s *Service) fetchRepoDB(ctx context.Context, repoName, arch string) (*repo.RemoteRepo, error) {
	dbURL := strings.TrimRight(s.cfg.Ayato.URL, "/") + "/repo/" + repoName + "/" + arch + "/" + repoName + ".db"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, dbURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := httpx.Default().Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch %s: status %d", dbURL, resp.StatusCode)
	}
	return repo.RemoteRepoFromDB(repoName, resp.Body)
}
