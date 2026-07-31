package cli

import (
	"fmt"
	"io"

	"github.com/Hayao0819/Kamisato/kayo/app"
)

func PrintUpdate(out io.Writer, result *app.UpdateResult) {
	resolved := result.Resolved
	fmt.Fprintf(out, "package: %s (source %s)\n", resolved.Pkgbase, resolved.Source)
	if result.Previous.Maintainer != resolved.Maintainer {
		fmt.Fprintf(out, "MAINTAINER CHANGED: %q -> %q\n", result.Previous.Maintainer, resolved.Maintainer)
	}
	if result.Previous.Commit == resolved.Commit {
		fmt.Fprintln(out, "no commit change since approval")
	} else {
		fmt.Fprintf(out, "commit: %s -> %s\n", Short(result.Previous.Commit), Short(resolved.Commit))
		if len(result.ChangedFiles) > 0 {
			fmt.Fprintln(out, "changed files:")
			for _, name := range result.ChangedFiles {
				fmt.Fprintf(out, "  %s\n", name)
			}
		}
	}
	PrintFindings(out, result.Report)
	if result.Approved {
		fmt.Fprintf(out, "re-pinned %s at %s\n", resolved.Pkgbase, Short(resolved.Commit))
	} else if !result.Approve {
		fmt.Fprintln(out, "(dry run; re-run with --approve to advance the pin)")
	}
}
