package shared

import (
	"github.com/Hayao0819/Kamisato/internal/blinkyutils"
	"github.com/spf13/cobra"
)

// AyatoServer is a resolved ayato endpoint: base URL plus credentials from the blinky server database.
type AyatoServer = blinkyutils.ServerInfo

// AddServerFlag registers the shared --server selection flag. One helper keeps
// the flag name, -s shorthand, and help text identical across every command that
// targets an ayato server, whether a leaf command or a group root whose children
// inherit it.
func AddServerFlag(cmd *cobra.Command) {
	cmd.PersistentFlags().StringP("server", "s", "", "ayato server (default: serverdb default)")
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

// ResolveAyatoServer looks up the base URL and credentials in the serverdb,
// using the default server when server is empty. This is the same store blinky
// uploads use, so a server from `ayaka server add` works here too.
func ResolveAyatoServer(server string) (*AyatoServer, error) {
	return blinkyutils.ResolveServer(server)
}
