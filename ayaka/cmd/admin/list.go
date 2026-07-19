package admincmd

import (
	"fmt"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/Hayao0819/Kamisato/ayaka/cmd/shared"
	"github.com/Hayao0819/Kamisato/internal/errors"
)

func adminListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List ayato admins",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			srv, err := shared.ServerFromFlag(cmd)
			if err != nil {
				return err
			}
			api, err := shared.AyatoClient(srv)
			if err != nil {
				return err
			}
			admins, err := api.ListAdmins(cmd.Context())
			if err != nil {
				return errors.WrapErr(err, "failed to list admins")
			}
			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "ID\tLOGIN")
			for _, a := range admins {
				fmt.Fprintf(w, "%d\t%s\n", a.ID, a.Login)
			}
			return w.Flush()
		},
	}
}
