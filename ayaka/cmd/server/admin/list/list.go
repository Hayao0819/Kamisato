package listcmd

import (
	"strconv"

	"github.com/Hayao0819/Kamisato/ayaka/cmd/internal/remote"
	cmdline "github.com/Hayao0819/Kamisato/internal/cli"
	"github.com/Hayao0819/Kamisato/internal/errors"
	"github.com/spf13/cobra"
)

type adminRow struct {
	ID    string `json:"id"`
	Login string `json:"login"`
}

const adminListDefaultFmt = "table {{.ID}}\t{{.Login}}"

func Cmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List ayato admins",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			srv, err := remote.DefaultProvider().ServerFromFlag(cmd)
			if err != nil {
				return err
			}
			api, err := remote.DefaultProvider().StoredClient(srv)
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
			format, err := cmdline.ResolveFormat(cmd, adminListDefaultFmt)
			if err != nil {
				return err
			}
			return cmdline.RenderList(cmd.OutOrStdout(), format, adminRow{ID: "ID", Login: "LOGIN"}, rows)
		},
	}
	cmdline.AddFormatFlags(cmd)
	return cmd
}
