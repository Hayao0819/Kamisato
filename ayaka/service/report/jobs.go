package report

import (
	"context"

	ayatostore "github.com/Hayao0819/Kamisato/internal/api/ayato/auth/store"
	"github.com/Hayao0819/Kamisato/internal/api/miko"
)

// FetchJobsBestEffort resolves the ayato client for the named or default
// server and lists recent jobs, for report columns that only enrich the
// output when a server is reachable. It returns nil (no error) when no
// registered server is available or the request fails, so callers stay
// offline-friendly instead of failing the whole command.
func FetchJobsBestEffort(server string) func() []miko.Job {
	return func() []miko.Job {
		srv, err := ayatostore.Resolve(server)
		if err != nil {
			return nil
		}
		api, err := ayatostore.NewStoredClient(srv)
		if err != nil {
			return nil
		}
		jobs, _ := api.ListJobs(context.Background())
		return jobs
	}
}
