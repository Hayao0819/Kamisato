package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/Hayao0819/Kamisato/internal/conf"
	"github.com/Hayao0819/Kamisato/internal/utils"
	"github.com/Hayao0819/Kamisato/pkg/aurweb"
	"github.com/Hayao0819/Kamisato/pkg/raiou"
	"github.com/Hayao0819/Kamisato/sara/audit"
	"github.com/Hayao0819/Kamisato/sara/trust"
	"github.com/spf13/cobra"
)

func loadConfig(cmd *cobra.Command) (*conf.SaraConfig, error) {
	configFile, _ := cmd.Flags().GetString("config")
	return conf.LoadSaraConfig(cmd.Flags(), configFile)
}

// resolved is an audit target reduced to the facts the trust model needs: where
// it came from, its pkgbase, the maintainer ACCOUNT that owns it, and the commit.
type resolved struct {
	Dir        string
	Source     string
	Pkgbase    string
	Maintainer string
	Commit     string
}

// resolve turns a target (a directory, a git URL, or an AUR package name) into a
// checked-out dir plus its provenance. cleanup must be called.
func resolve(ctx context.Context, cfg *conf.SaraConfig, target, ref string) (resolved, func(), error) {
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
			return resolved{}, func() {}, err
		}
	}

	commit, _ := audit.HeadCommit(ctx, dir)
	r := resolved{Dir: dir, Source: source, Pkgbase: readPkgbase(dir, target), Commit: commit}
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
func aurMeta(ctx context.Context, cfg *conf.SaraConfig, name, pkgbase string) (maintainer, base string) {
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

func auditCmd() *cobra.Command {
	var ref string
	cmd := &cobra.Command{
		Use:   "audit <package|dir|git-url>",
		Short: "Statically audit a PKGBUILD and check maintainer trust",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig(cmd)
			if err != nil {
				return err
			}

			r, cleanup, err := resolve(cmd.Context(), cfg, args[0], ref)
			defer cleanup()
			if err != nil {
				return err
			}

			report, err := audit.Scan(r.Dir)
			if err != nil {
				return err
			}
			store, err := trust.Open(cfg.ResolvedTrustStore())
			if err != nil {
				return err
			}
			verdict := store.Evaluate(r.Source, r.Pkgbase, r.Maintainer)

			printReport(cmd.OutOrStdout(), r, report, verdict)
			if report.Max() >= audit.SevHigh {
				return utils.NewErr("audit found high-severity issues")
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&ref, "ref", "", "git ref or commit to check out")
	return cmd
}

func printReport(w io.Writer, r resolved, report audit.Report, verdict trust.Verdict) {
	fmt.Fprintf(w, "package:    %s\n", r.Pkgbase)
	fmt.Fprintf(w, "source:     %s\n", r.Source)
	maintainer := r.Maintainer
	if maintainer == "" {
		maintainer = "(orphan/unknown)"
	}
	fmt.Fprintf(w, "maintainer: %s\n", maintainer)
	if r.Commit != "" {
		fmt.Fprintf(w, "commit:     %s\n", r.Commit)
	}
	fmt.Fprintf(w, "trust:      %s", verdict.Decision)
	if len(verdict.Reasons) > 0 {
		fmt.Fprintf(w, " (%s)", strings.Join(verdict.Reasons, "; "))
	}
	fmt.Fprintln(w)

	printFindings(w, report)
}

func printFindings(w io.Writer, report audit.Report) {
	if len(report.Findings) == 0 {
		fmt.Fprintln(w, "findings:   none")
		return
	}
	fmt.Fprintln(w, "findings:")
	for _, f := range report.Findings {
		fmt.Fprintf(w, "  [%s] %s: %s — %s\n", f.Severity, f.Code, f.Title, f.Detail)
	}
}
