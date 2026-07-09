package service

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/Hayao0819/Kamisato/internal/conf"
	"github.com/Hayao0819/Kamisato/internal/errors"
	"github.com/Hayao0819/Kamisato/pkg/aurweb"
	"github.com/Hayao0819/Kamisato/pkg/pacman/depend"
)

// maintainerLookup is the slice of the aurweb upstream the trust gate needs:
// the info RPC, from which each package's Maintainer is read. Kept as an
// interface so tests inject a fake and never hit the network.
type maintainerLookup interface {
	Info(ctx context.Context, names []string) ([]aurweb.Pkg, error)
}

// checkDepTrust evaluates the build-time AUR trust policy for one resolved
// dependency before it is built. It fetches the dep's maintainer from the AUR
// and returns an error only when the policy blocks the dep, so the build fails
// closed. The target the user submitted is never passed here — only its
// transitive AUR deps are gated, since a malicious dep would otherwise be built
// and published silently.
func (s *Service) checkDepTrust(ctx context.Context, up maintainerLookup, dep depend.Pkg) error {
	maintainer, err := depMaintainer(ctx, up, dep)
	if err != nil {
		return errors.WrapErr(err, "failed to look up AUR maintainer for "+dep.PackageBase)
	}

	switch s.cfg.AURTrust.Decide(dep.PackageBase, maintainer) {
	case conf.AURTrustByPkgbase:
		slog.Info("AUR dependency trusted via pkgbase allowlist", "pkgbase", dep.PackageBase, "maintainer", maintainerLabel(maintainer))
		return nil
	case conf.AURTrustByMaintainer:
		slog.Info("AUR dependency trusted via maintainer", "pkgbase", dep.PackageBase, "maintainer", maintainer)
		return nil
	case conf.AURTrustUntrusted:
		slog.Warn("building untrusted AUR dependency because allow_untrusted is set", "pkgbase", dep.PackageBase, "maintainer", maintainerLabel(maintainer))
		return nil
	default:
		slog.Error("blocked untrusted AUR dependency", "pkgbase", dep.PackageBase, "maintainer", maintainerLabel(maintainer))
		return fmt.Errorf("AUR dependency %s (maintainer %s) is not trusted; add it to miko AUR trust (trusted_pkgbases/trusted_maintainers) or set allow_untrusted", dep.PackageBase, maintainerLabel(maintainer))
	}
}

// depMaintainer returns the AUR maintainer of dep's package base. An empty
// string means the package is orphaned (or no longer in the AUR), which the
// trust policy treats as untrusted.
func depMaintainer(ctx context.Context, up maintainerLookup, dep depend.Pkg) (string, error) {
	infos, err := up.Info(ctx, []string{dep.Name})
	if err != nil {
		return "", err
	}
	for _, p := range infos {
		if p.Name == dep.Name || p.PackageBase == dep.PackageBase {
			return p.Maintainer, nil
		}
	}
	return "", nil
}

// maintainerLabel renders an empty (orphaned) maintainer for log and error text.
func maintainerLabel(m string) string {
	if m == "" {
		return "<orphaned>"
	}
	return m
}
