package shared

import (
	"github.com/spf13/cobra"

	"github.com/Hayao0819/Kamisato/internal/serverstore"
)

// AyatoServer is a resolved Ayato endpoint and credential source.
type AyatoServer = serverstore.Endpoint

const serverFlagHelp = "ayato server (default: serverdb default)"

// AddServerFlag registers the shared --server selection flag on a leaf command.
// One helper keeps the flag name, -s shorthand, and help text identical across
// every command that targets an ayato server.
func AddServerFlag(cmd *cobra.Command) {
	cmd.Flags().StringP("server", "s", "", serverFlagHelp)
}

// AddPersistentServerFlag is AddServerFlag for group roots whose children
// inherit the flag.
func AddPersistentServerFlag(cmd *cobra.Command) {
	cmd.PersistentFlags().StringP("server", "s", "", serverFlagHelp)
}

// ServerFromFlag reads --server and resolves it through ResolveAyatoServer. This
// is the single resolution path for every command that targets an ayato server.
func ServerFromFlag(cmd *cobra.Command) (*AyatoServer, error) {
	server, err := cmd.Flags().GetString("server")
	if err != nil {
		return nil, err
	}
	return ResolveAyatoServer(server)
}

// ResolveAyatoServer resolves a named or default server.
func ResolveAyatoServer(server string) (*AyatoServer, error) {
	return serverstore.Resolve(server)
}
