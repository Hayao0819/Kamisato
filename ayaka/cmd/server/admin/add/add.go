package addcmd

import (
	"fmt"

	"github.com/Hayao0819/Kamisato/ayaka/cmd/internal/remote"
	"github.com/Hayao0819/Kamisato/ayaka/cmd/server/admin/internal/userid"
	"github.com/Hayao0819/Kamisato/internal/errors"
	"github.com/spf13/cobra"
)

func Cmd() *cobra.Command {
	return &cobra.Command{
		Use:   "add <login-or-id>",
		Short: "Add an ayato admin by GitHub login or numeric id",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			srv, err := remote.DefaultProvider().ServerFromFlag(cmd)
			if err != nil {
				return err
			}
			id, login := userid.Parse(args[0])
			api, err := remote.DefaultProvider().StoredClient(srv)
			if err != nil {
				return err
			}
			admin, err := api.AddAdmin(cmd.Context(), id, login)
			if err != nil {
				return errors.WrapErr(err, "failed to add admin")
			}
			fmt.Fprintf(cmd.OutOrStdout(), "added %s (%d)\n", admin.Login, admin.ID)
			return nil
		},
	}
}
