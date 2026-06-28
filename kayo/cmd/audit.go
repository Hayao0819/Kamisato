package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/Hayao0819/Kamisato/internal/conf"
	"github.com/Hayao0819/Kamisato/internal/llmaudit"
	"github.com/Hayao0819/Kamisato/internal/utils"
	"github.com/Hayao0819/Kamisato/kayo/audit"
	"github.com/Hayao0819/Kamisato/kayo/trust"
	"github.com/Hayao0819/Kamisato/pkg/aurweb"
	"github.com/Hayao0819/Kamisato/pkg/raiou"
	"github.com/spf13/cobra"
)

func loadConfig(cmd *cobra.Command) (*conf.KayoConfig, error) {
	configFile, _ := cmd.Flags().GetString("config")
	return conf.LoadKayoConfig(cmd.Flags(), configFile)
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
func resolve(ctx context.Context, cfg *conf.KayoConfig, target, ref string) (resolved, func(), error) {
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

func auditCmd() *cobra.Command {
	var ref string
	var llm bool
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

			out := cmd.OutOrStdout()
			printReport(out, r, report, verdict)
			printLLMAdvisory(cmd.Context(), out, cfg, r.Dir, llm)
			if report.Max() >= audit.SevHigh {
				return utils.NewErr("audit found high-severity issues")
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&ref, "ref", "", "git ref or commit to check out")
	// Not "--llm": that flag name collides with the [llm] config section and the
	// loader would try to decode a bool onto the struct.
	cmd.Flags().BoolVar(&llm, "llm-advisory", false, "also run the LLM advisory pass (overrides config)")
	return cmd
}

// printLLMAdvisory runs the optional, advisory-only LLM triage and prints it.
// Strictly best-effort: any failure prints a note and is swallowed, never
// affecting the audit's verdict or exit code — an LLM is nondeterministic and
// prompt-injectable, so it must not gate anything.
func printLLMAdvisory(ctx context.Context, w io.Writer, cfg *conf.KayoConfig, dir string, force bool) {
	if !cfg.LLM.Enabled && !force {
		return
	}
	pkgbuild, err := os.ReadFile(filepath.Join(dir, "PKGBUILD"))
	if err != nil {
		fmt.Fprintf(w, "llm:        skipped (%v)\n", err)
		return
	}
	// Send every .install scriptlet, not just the first: a split-package recipe
	// ships several, and a malicious one could hide in a later-sorting file.
	var install strings.Builder
	matches, _ := filepath.Glob(filepath.Join(dir, "*.install"))
	for _, m := range matches {
		if b, err := os.ReadFile(m); err == nil {
			fmt.Fprintf(&install, "--- %s ---\n%s\n", filepath.Base(m), b)
		}
	}

	model, err := llmaudit.NewModel(cfg.LLM.Provider, cfg.LLM.Model, cfg.LLM.BaseURL)
	if err != nil {
		fmt.Fprintf(w, "llm:        unavailable (%v)\n", err)
		return
	}
	adv, err := llmaudit.Advise(ctx, model, string(pkgbuild), install.String())
	if err != nil {
		fmt.Fprintf(w, "llm:        advisory failed (%v)\n", err)
		return
	}
	printAdvisory(w, adv)
}

func printAdvisory(w io.Writer, a *llmaudit.Advisory) {
	fmt.Fprintf(w, "llm advisory: risk=%s (not a gate)\n", sanitizeLLM(a.Risk))
	if a.Summary != "" {
		fmt.Fprintf(w, "  %s\n", sanitizeLLM(a.Summary))
	}
	for _, f := range a.Findings {
		fmt.Fprintf(w, "  [%s] %s — %s\n", sanitizeLLM(f.Severity), sanitizeLLM(f.Title), sanitizeLLM(f.Detail))
	}
}

// sanitizeLLM strips control characters from model output before it reaches the
// terminal: the text is steered by the attacker-controlled recipe and could
// carry ANSI escapes to repaint the screen or forge a clean verdict.
func sanitizeLLM(s string) string {
	return strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f || (r >= 0x80 && r <= 0x9f) {
			return -1
		}
		return r
	}, s)
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
