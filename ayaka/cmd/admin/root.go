package admincmd

import (
	"github.com/Hayao0819/Kamisato/ayaka/cmd/shared"
	"github.com/spf13/cobra"
)

func Cmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "admin",
		Short: "Manage ayato admin allowlist",
		Long:  "List, add, and remove ayato admins. Requires a logged-in server with a CLI token.",
	}
	cmd.PersistentFlags().StringP("server", "s", "", "ayato server (default: serverdb default)")

	cmd.AddCommand(adminListCmd(), adminAddCmd(), adminRemoveCmd())
	return cmd
}

func resolveAdminServer(cmd *cobra.Command) (*shared.AyatoServer, error) {
	server, err := cmd.Flags().GetString("server")
	if err != nil {
		return nil, err
	}
	return shared.ResolveAyatoServer(server)
}
