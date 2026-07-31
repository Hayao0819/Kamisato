package app

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"

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

type resolvePackage func(context.Context, string) (aurweb.Pkg, string, bool, bool)
type lookupPackages func(context.Context, []string) ([]aurweb.Pkg, error)

type verifyDependencies struct {
	openStore      func(string) (*trust.Store, error)
	newResolver    func(context.Context, *kayoconfig.KayoConfig) (resolvePackage, error)
	newUpstream    func(*kayoconfig.KayoConfig) lookupPackages
	syncPackages   func() (map[string]bool, error)
	checkCloneHead func(context.Context, string, string, string) (clonecache.Result, error)
}

func Verify(ctx context.Context, cfg *kayoconfig.KayoConfig, names []string, strict bool, out io.Writer) error {
	return verify(defaultVerifyDependencies(), ctx, cfg, names, strict, out)
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

func verify(deps verifyDependencies, ctx context.Context, cfg *kayoconfig.KayoConfig, names []string, strict bool, out io.Writer) error {
	if len(names) == 0 {
		return nil
	}
	if out == nil {
		out = io.Discard
	}

	store, err := deps.openStore(cfg.ResolvedTrustStore())
	if err != nil {
		return err
	}
	resolve, err := deps.newResolver(ctx, cfg)
	if err != nil {
		return err
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
					return errors.WrapErr(infoErr, "could not verify AUR packages upstream (failing closed)")
				}
				slog.Warn("upstream lookup failed; AUR packages unverified this run", "error", infoErr)
			}
			for _, pkg := range packages {
				found[pkg.Name] = verifyTarget{pkg: pkg, source: "aur"}
			}
		}
	}

	needsReview := reportTrust(out, store, names, found)
	if reportCloneDrift(ctx, out, store, cfg.ResolvedYayCacheDir(), names, found, deps.checkCloneHead) {
		needsReview = true
	}
	if needsReview && enforce {
		return errors.NewErr("untrusted packages in transaction; review with 'kayo update' or 'kayo trust add'")
	}
	return nil
}

func reportTrust(w io.Writer, store *trust.Store, order []string, found map[string]verifyTarget) (needsReview bool) {
	for _, name := range order {
		target, ok := found[name]
		if !ok {
			continue
		}
		verdict := store.EvaluateResolved(target.source, target.pkg.PackageBase, target.pkg.Maintainer, target.delegatedVerified)
		if verdict.Decision == trust.Trusted {
			fmt.Fprintf(w, "  ok      %s (%s)\n", name, target.source)
			continue
		}
		fmt.Fprintf(w, "  REVIEW  %s (%s) — %s\n", name, target.source, strings.Join(verdict.Reasons, "; "))
		needsReview = true
	}
	return needsReview
}

func reportCloneDrift(
	ctx context.Context,
	w io.Writer,
	store *trust.Store,
	root string,
	order []string,
	found map[string]verifyTarget,
	checkCloneHead func(context.Context, string, string, string) (clonecache.Result, error),
) (drifted bool) {
	for _, name := range order {
		target, ok := found[name]
		if !ok {
			continue
		}
		base := target.pkg.PackageBase
		approval, ok := store.Approval(base)
		if !ok || approval.Commit == "" {
			continue
		}
		result, err := checkCloneHead(ctx, root, base, approval.Commit)
		if err != nil {
			slog.Warn("could not read clone cache; skipping pin check", "pkgbase", base, "error", err)
			continue
		}
		if result.Drifted() {
			fmt.Fprintf(w, "  DRIFT   %s (%s) — clone cache at %s, approved %s\n", name, base, shortCommit(result.Head), shortCommit(approval.Commit))
			drifted = true
		}
	}
	return drifted
}

func shortCommit(commit string) string {
	if len(commit) > 12 {
		return commit[:12]
	}
	return commit
}
