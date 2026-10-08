package report

import (
	"context"

	ayatostore "github.com/Hayao0819/Kamisato/ayato/client/auth/store"
	miko "github.com/Hayao0819/Kamisato/miko/client"
)

// FetchJobsBestEffort resolves the ayato client for the named or default
// server and lists recent jobs, for report columns that only enrich the
// output when a server is reachable. It returns nil (no error) when no
// registered server is available or the request fails, so callers stay
// offline-friendly instead of failing the whole command.
func FetchJobsBestEffort(ctx context.Context, server string) func() []miko.Job {
	return func() []miko.Job {
		srv, err := ayatostore.Resolve(server)
		if err != nil {
			return nil
		}
		api, err := ayatostore.NewStoredClient(srv)
		if err != nil {
			return nil
		}
		jobs, _ := api.ListJobs(ctx)
		return jobs
	}
}
