// Package pruning adapts the shared prune workflow to command output.
package pruning

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/Hayao0819/Kamisato/ayaka/service/plan"
	"github.com/Hayao0819/Kamisato/ayaka/source"
)

func Run(ctx context.Context, out io.Writer, client *http.Client, src *source.SourceRepo, options plan.PruneOptions) error {
	packages, err := plan.Prune(ctx, client, src, options)
	if err != nil {
		return err
	}
	if options.DryRun {
		for _, name := range packages {
			fmt.Fprintln(out, name)
		}
	}
	return nil
}
