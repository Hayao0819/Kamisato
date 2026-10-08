package verifycmd

import (
	"fmt"
	"io"
	"strings"

	"github.com/Hayao0819/Kamisato/kayo/cmd/internal/output"
	"github.com/Hayao0819/Kamisato/kayo/service"
	"github.com/Hayao0819/Kamisato/kayo/trust"
)

func printResults(w io.Writer, results []service.Verification) {
	for _, result := range results {
		if result.Verdict.Decision == trust.Trusted {
			fmt.Fprintf(w, "  ok      %s (%s)\n", result.Name, result.Source)
		} else {
			fmt.Fprintf(w, "  REVIEW  %s (%s) — %s\n", result.Name, result.Source, strings.Join(result.Verdict.Reasons, "; "))
		}
	}
	for _, result := range results {
		if result.Clone.Drifted() {
			fmt.Fprintf(w, "  DRIFT   %s (%s) — clone cache at %s, approved %s\n", result.Name, result.Pkgbase, output.Short(result.Clone.Head), output.Short(result.Clone.Pinned))
		}
	}
}
