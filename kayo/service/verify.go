package service

import (
	"context"
	"log/slog"

	"github.com/Hayao0819/Kamisato/internal/errors"
	pacmanhost "github.com/Hayao0819/Kamisato/internal/pacman/host"
	"github.com/Hayao0819/Kamisato/kayo/clonecache"
	kayoconfig "github.com/Hayao0819/Kamisato/kayo/config"
	"github.com/Hayao0819/Kamisato/kayo/trust"
	"github.com/Hayao0819/Kamisato/pkg/aurweb"
)

type verifyTarget struct {
	pkg               aurweb.Pkg
	source            string
	delegatedVerified bool
}

// Verification is the trust and clone-pin decision for one managed package.
// Official-repository packages are absent from the result.
type Verification struct {
	Name    string
	Source  string
	Pkgbase string
	Verdict trust.Verdict
	Clone   clonecache.Result
}

type resolvePackage func(context.Context, string) (aurweb.Pkg, string, bool, bool)
type lookupPackages func(context.Context, []string) ([]aurweb.Pkg, error)

type verifyDependencies struct {
	openStore      func(string) (*trust.Store, error)
	newResolver    func(context.Context, *kayoconfig.KayoConfig) (resolvePackage, error)
	newUpstream    func(*kayoconfig.KayoConfig) lookupPackages
	syncPackages   func() (map[string]bool, error)
	checkCloneHead func(context.Context, string, string, string) (clonecache.Result, error)
}

func Verify(ctx context.Context, cfg *kayoconfig.KayoConfig, names []string, strict bool) ([]Verification, error) {
	return verify(defaultVerifyDependencies(), ctx, cfg, names, strict)
}

func defaultVerifyDependencies() verifyDependencies {
	return verifyDependencies{
		openStore: trust.Open,
		newResolver: func(ctx context.Context, cfg *kayoconfig.KayoConfig) (resolvePackage, error) {
			composite, _, err := BuildComposite(ctx, cfg)
			if err != nil {
				return nil, err
			}
			return composite.Resolve, nil
		},
		newUpstream: func(cfg *kayoconfig.KayoConfig) lookupPackages {
			upstream := UpstreamClient(cfg)
			if upstream == nil {
				return nil
			}
			return upstream.Info
		},
		syncPackages:   pacmanhost.SyncPackages,
		checkCloneHead: clonecache.Check,
	}
}

func verify(deps verifyDependencies, ctx context.Context, cfg *kayoconfig.KayoConfig, names []string, strict bool) ([]Verification, error) {
	if len(names) == 0 {
		return nil, nil
	}

	store, err := deps.openStore(cfg.ResolvedTrustStore())
	if err != nil {
		return nil, err
	}
	resolve, err := deps.newResolver(ctx, cfg)
	if err != nil {
		return nil, err
	}
	upstream := deps.newUpstream(cfg)
	enforce := strict || cfg.ResolvedEnforceMode() == "enforce"

	found := make(map[string]verifyTarget, len(names))
	var unknown []string
	for _, name := range names {
		if pkg, source, delegatedVerified, ok := resolve(ctx, name); ok {
			found[name] = verifyTarget{pkg: pkg, source: source, delegatedVerified: delegatedVerified}
		} else {
			unknown = append(unknown, name)
		}
	}

	if len(unknown) > 0 && upstream != nil {
		foreign := unknown
		if syncPackages, syncErr := deps.syncPackages(); syncErr == nil {
			foreign = foreign[:0]
			for _, name := range unknown {
				if !syncPackages[name] {
					foreign = append(foreign, name)
				}
			}
		} else {
			slog.Warn("could not list sync-repo packages; treating all unknown targets as foreign", "error", syncErr)
		}
		if len(foreign) > 0 {
			packages, infoErr := upstream(ctx, foreign)
			if infoErr != nil {
				if enforce {
					return nil, errors.WrapErr(infoErr, "could not verify AUR packages upstream (failing closed)")
				}
				slog.Warn("upstream lookup failed; AUR packages unverified this run", "error", infoErr)
			}
			for _, pkg := range packages {
				found[pkg.Name] = verifyTarget{pkg: pkg, source: "aur"}
			}
		}
	}

	results, needsReview := evaluateTrust(store, names, found)
	if checkCloneDrift(ctx, store, cfg.ResolvedYayCacheDir(), results, deps.checkCloneHead) {
		needsReview = true
	}
	if needsReview && enforce {
		return results, errors.NewErr("untrusted packages in transaction; review with 'kayo update' or 'kayo trust add'")
	}
	return results, nil
}

func evaluateTrust(store *trust.Store, order []string, found map[string]verifyTarget) (results []Verification, needsReview bool) {
	for _, name := range order {
		target, ok := found[name]
		if !ok {
			continue
		}
		verdict := store.EvaluateResolved(target.source, target.pkg.PackageBase, target.pkg.Maintainer, target.delegatedVerified)
		results = append(results, Verification{Name: name, Source: target.source, Pkgbase: target.pkg.PackageBase, Verdict: verdict})
		if verdict.Decision != trust.Trusted {
			needsReview = true
		}
	}
	return results, needsReview
}

func checkCloneDrift(
	ctx context.Context,
	store *trust.Store,
	root string,
	results []Verification,
	checkCloneHead func(context.Context, string, string, string) (clonecache.Result, error),
) (drifted bool) {
	for i := range results {
		base := results[i].Pkgbase
		approval, ok := store.Approval(base)
		if !ok || approval.Commit == "" {
			continue
		}
		result, err := checkCloneHead(ctx, root, base, approval.Commit)
		if err != nil {
			slog.Warn("could not read clone cache; skipping pin check", "pkgbase", base, "error", err)
			continue
		}
		result.Pinned = approval.Commit
		results[i].Clone = result
		if result.Drifted() {
			drifted = true
		}
	}
	return drifted
}
