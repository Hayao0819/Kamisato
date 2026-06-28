package shared

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/Hayao0819/Kamisato/internal/conf"
	"github.com/Hayao0819/Kamisato/internal/llmaudit"
	"github.com/Hayao0819/Kamisato/kayo/audit"
	"github.com/Hayao0819/Kamisato/kayo/trust"
)

// PrintLLMAdvisory runs the optional, advisory-only LLM triage and prints it.
// Strictly best-effort: any failure prints a note and is swallowed, never
// affecting the audit's verdict or exit code — an LLM is nondeterministic and
// prompt-injectable, so it must not gate anything.
func PrintLLMAdvisory(ctx context.Context, w io.Writer, cfg *conf.KayoConfig, dir string, force bool) {
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

func PrintReport(w io.Writer, r Resolved, report audit.Report, verdict trust.Verdict) {
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

	PrintFindings(w, report)
}

func PrintFindings(w io.Writer, report audit.Report) {
	if len(report.Findings) == 0 {
		fmt.Fprintln(w, "findings:   none")
		return
	}
	fmt.Fprintln(w, "findings:")
	for _, f := range report.Findings {
		fmt.Fprintf(w, "  [%s] %s: %s — %s\n", f.Severity, f.Code, f.Title, f.Detail)
	}
}
