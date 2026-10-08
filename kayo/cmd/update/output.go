package updatecmd

import (
	"fmt"
	"io"

	"github.com/Hayao0819/Kamisato/kayo/cmd/internal/output"
	"github.com/Hayao0819/Kamisato/kayo/service"
)

func printUpdate(out io.Writer, result *service.UpdateResult) {
	resolved := result.Resolved
	fmt.Fprintf(out, "package: %s (source %s)\n", resolved.Pkgbase, resolved.Source)
	if result.Previous.Maintainer != resolved.Maintainer {
		fmt.Fprintf(out, "MAINTAINER CHANGED: %q -> %q\n", result.Previous.Maintainer, resolved.Maintainer)
	}
	if result.Previous.Commit == resolved.Commit {
		fmt.Fprintln(out, "no commit change since approval")
	} else {
		fmt.Fprintf(out, "commit: %s -> %s\n", output.Short(result.Previous.Commit), output.Short(resolved.Commit))
		if len(result.ChangedFiles) > 0 {
			fmt.Fprintln(out, "changed files:")
			for _, name := range result.ChangedFiles {
				fmt.Fprintf(out, "  %s\n", name)
			}
		}
	}
	output.PrintFindings(out, result.Report)
	if result.Approved {
		fmt.Fprintf(out, "re-pinned %s at %s\n", resolved.Pkgbase, output.Short(resolved.Commit))
	} else if !result.Approve {
		fmt.Fprintln(out, "(dry run; re-run with --approve to advance the pin)")
	}
}
