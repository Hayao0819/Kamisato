package output

import (
	"fmt"
	"io"
	"strings"

	"github.com/Hayao0819/Kamisato/kayo/audit"
	"github.com/Hayao0819/Kamisato/kayo/audit/llm"
	"github.com/Hayao0819/Kamisato/kayo/service"
	"github.com/Hayao0819/Kamisato/kayo/trust"
)

// PrintLLMAdvisory only renders a result. An advisory failure is displayed but
// must never change the trust verdict or the command's exit status.
func PrintLLMAdvisory(w io.Writer, adv *llm.Advisory, err error) {
	if err != nil {
		fmt.Fprintf(w, "llm:        %v\n", err)
		return
	}
	if adv != nil {
		printAdvisory(w, adv)
	}
}

func printAdvisory(w io.Writer, a *llm.Advisory) {
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

func PrintReport(w io.Writer, r service.Resolved, report audit.Report, verdict trust.Verdict) {
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

func Short(commit string) string {
	if len(commit) > 12 {
		return commit[:12]
	}
	return commit
}
