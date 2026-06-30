package mikocmd

import (
	"github.com/Hayao0819/Kamisato/internal/blinkyutils"
	"github.com/spf13/cobra"
)

// resolveJobBase returns the ayato base URL for the job endpoints, which are
// public and need no credentials. --server, else the serverdb default.
func resolveJobBase(cmd *cobra.Command) (string, error) {
	server, err := cmd.Flags().GetString("server")
	if err != nil {
		return "", err
	}
	return blinkyutils.ResolveServerName(server)
}
