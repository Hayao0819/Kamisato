package report

import (
	"context"

	"github.com/Hayao0819/Kamisato/internal/client"
	"github.com/Hayao0819/Kamisato/internal/serverstore"
)

// FetchJobsBestEffort resolves the ayato client for the named or default
// server and lists recent jobs, for report columns that only enrich the
// output when a server is reachable. It returns nil (no error) when no
// registered server is available or the request fails, so callers stay
// offline-friendly instead of failing the whole command.
func FetchJobsBestEffort(server string) func() []client.Job {
	return func() []client.Job {
		srv, err := serverstore.Resolve(server)
		if err != nil {
			return nil
		}
		api, err := serverstore.NewClient(srv)
		if err != nil {
			return nil
		}
		jobs, _ := api.ListJobs(context.Background())
		return jobs
	}
}
