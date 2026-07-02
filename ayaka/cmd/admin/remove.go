package admincmd

import (
	"fmt"
	"strconv"

	"github.com/Hayao0819/Kamisato/ayaka/cmd/shared"
	"github.com/Hayao0819/Kamisato/internal/ayatoclient"
	"github.com/Hayao0819/Kamisato/internal/errwrap"
	"github.com/spf13/cobra"
)

func adminRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "remove <id>",
		Short: "Remove an ayato admin by numeric id",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			srv, err := shared.ServerFromFlag(cmd)
			if err != nil {
				return err
			}
			id, perr := strconv.ParseInt(args[0], 10, 64)
			if perr != nil || id <= 0 {
				return errwrap.NewErrf("invalid id: %s", args[0])
			}
			if err := ayatoclient.RemoveAdmin(cmd.Context(), srv.URL, srv.Password, id); err != nil {
				return errwrap.WrapErr(err, "failed to remove admin")
			}
			fmt.Fprintf(cmd.OutOrStdout(), "removed admin %d\n", id)
			return nil
		},
	}
}
