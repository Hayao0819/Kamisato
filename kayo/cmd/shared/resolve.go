package shared

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/Hayao0819/Kamisato/internal/conf"
	"github.com/Hayao0819/Kamisato/kayo/audit"
	"github.com/Hayao0819/Kamisato/pkg/aurweb"
	"github.com/Hayao0819/Kamisato/pkg/raiou"
)

// Resolved is an audit target reduced to the facts the trust model needs: where
// it came from, its pkgbase, the maintainer ACCOUNT that owns it, and the commit.
type Resolved struct {
	Dir        string
	Source     string
	Pkgbase    string
	Maintainer string
	Commit     string
}

// Resolve turns a target (a directory, a git URL, or an AUR package name) into a
// checked-out dir plus its provenance. cleanup must be called.
func Resolve(ctx context.Context, cfg *conf.KayoConfig, target, ref string) (Resolved, func(), error) {
	cleanup := func() {}

	var dir, source string
	if st, statErr := os.Stat(target); statErr == nil && st.IsDir() {
		dir, source = target, "local"
	} else {
		url := target
		source = target
		if !strings.Contains(target, "://") && !strings.HasSuffix(target, ".git") {
			url = cfg.AURGitBase() + "/" + target + ".git"
			source = "aur"
		}
		var err error
		dir, cleanup, err = audit.Clone(ctx, url, ref)
		if err != nil {
			return Resolved{}, func() {}, err
		}
	}

	commit, _ := audit.HeadCommit(ctx, dir)
	r := Resolved{Dir: dir, Source: source, Pkgbase: readPkgbase(dir, target), Commit: commit}
	if source == "aur" {
		r.Maintainer, r.Pkgbase = aurMeta(ctx, cfg, target, r.Pkgbase)
	}
	return r, cleanup, nil
}

// readPkgbase parses the .SRCINFO pkgbase, falling back to the target's basename.
func readPkgbase(dir, fallback string) string {
	if si, err := raiou.ParseSrcinfoFile(filepath.Join(dir, ".SRCINFO")); err == nil && si.PkgBase != "" {
		return si.PkgBase
	}
	return filepath.Base(fallback)
}

// aurMeta best-effort fetches the maintainer account (and authoritative pkgbase)
// for an AUR package from the upstream RPC. The maintainer account, not any git
// email, is the trust anchor.
func aurMeta(ctx context.Context, cfg *conf.KayoConfig, name, pkgbase string) (maintainer, base string) {
	up := aurweb.NewAURUpstream(cfg.Upstream.RPCURL, aurweb.WithGitBase(cfg.AURGitBase()))
	pkgs, err := up.Info(ctx, []string{name})
	if err != nil || len(pkgs) == 0 {
		return "", pkgbase
	}
	base = pkgbase
	if pkgs[0].PackageBase != "" {
		base = pkgs[0].PackageBase
	}
	return pkgs[0].Maintainer, base
}
