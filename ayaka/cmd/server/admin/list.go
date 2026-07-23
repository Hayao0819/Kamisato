package admincmd

import (
	"strconv"

	"github.com/spf13/cobra"

	"github.com/Hayao0819/Kamisato/ayaka/cmd/shared"
	"github.com/Hayao0819/Kamisato/internal/cliutil"
	"github.com/Hayao0819/Kamisato/internal/errors"
)

type adminRow struct {
	ID    string `json:"id"`
	Login string `json:"login"`
}

const adminListDefaultFmt = "table {{.ID}}\t{{.Login}}"

func adminListCmd() *cobra.Command {
	cmd := &cobra.Command{
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
			rows := make([]adminRow, 0, len(admins))
			for _, a := range admins {
				rows = append(rows, adminRow{ID: strconv.FormatInt(a.ID, 10), Login: a.Login})
			}
			format, err := cliutil.ResolveFormat(cmd, adminListDefaultFmt)
			if err != nil {
				return err
			}
			return cliutil.RenderList(cmd.OutOrStdout(), format, adminRow{ID: "ID", Login: "LOGIN"}, rows)
		},
	}
	cliutil.AddFormatFlags(cmd)
	return cmd
}
