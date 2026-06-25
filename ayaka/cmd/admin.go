package cmd

import (
	"fmt"
	"strconv"
	"text/tabwriter"

	"github.com/Hayao0819/Kamisato/internal/ayatoclient"
	"github.com/Hayao0819/Kamisato/internal/utils"
	"github.com/spf13/cobra"
)

func adminCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "admin",
		Short: "Manage ayato admin allowlist",
		Long:  "List, add, and remove ayato admins. Requires a logged-in server with a CLI token.",
	}
	cmd.PersistentFlags().StringP("server", "s", "", "ayato server (default: serverdb default)")

	cmd.AddCommand(adminListCmd(), adminAddCmd(), adminRemoveCmd())
	return cmd
}

func adminListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List ayato admins",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			srv, err := resolveAdminServer(cmd)
			if err != nil {
				return err
			}
			admins, err := ayatoclient.ListAdmins(srv.URL, srv.Password)
			if err != nil {
				return utils.WrapErr(err, "failed to list admins")
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

func adminAddCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "add <login-or-id>",
		Short: "Add an ayato admin by GitHub login or numeric id",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			srv, err := resolveAdminServer(cmd)
			if err != nil {
				return err
			}
			var id int64
			var login string
			if n, perr := strconv.ParseInt(args[0], 10, 64); perr == nil && n > 0 {
				id = n
			} else {
				login = args[0]
			}
			admin, err := ayatoclient.AddAdmin(srv.URL, srv.Password, id, login)
			if err != nil {
				return utils.WrapErr(err, "failed to add admin")
			}
			fmt.Fprintf(cmd.OutOrStdout(), "added %s (%d)\n", admin.Login, admin.ID)
			return nil
		},
	}
}

func adminRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "remove <id>",
		Short: "Remove an ayato admin by numeric id",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			srv, err := resolveAdminServer(cmd)
			if err != nil {
				return err
			}
			id, perr := strconv.ParseInt(args[0], 10, 64)
			if perr != nil || id <= 0 {
				return utils.NewErrf("invalid id: %s", args[0])
			}
			if err := ayatoclient.RemoveAdmin(srv.URL, srv.Password, id); err != nil {
				return utils.WrapErr(err, "failed to remove admin")
			}
			fmt.Fprintf(cmd.OutOrStdout(), "removed admin %d\n", id)
			return nil
		},
	}
}

func resolveAdminServer(cmd *cobra.Command) (*ayatoServer, error) {
	server, err := cmd.Flags().GetString("server")
	if err != nil {
		return nil, err
	}
	return resolveAyatoServer(server)
}

func init() {
	subCmds.Add(adminCmd())
}
