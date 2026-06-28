package mikocmd

import (
	blinky_util "github.com/BrenekH/blinky/cmd/blinky/util"
	"github.com/Hayao0819/Kamisato/ayaka/cmd/shared"
	"github.com/Hayao0819/Kamisato/internal/utils"
	"github.com/spf13/cobra"
)

// resolveJobBase returns the ayato base URL for the job endpoints, which are
// public and need no credentials. It honors the miko --server flag and falls
// back to the serverdb default.
func resolveJobBase(cmd *cobra.Command) (string, error) {
	server, err := cmd.Flags().GetString("server")
	if err != nil {
		return "", err
	}

	db, err := blinky_util.ReadServerDB()
	if err != nil {
		return "", utils.WrapErr(err, "failed to read server database")
	}
	if server == "" {
		server = db.DefaultServer
	}
	if server == "" {
		return "", shared.ErrNoServerSpecified
	}
	return server, nil
}
