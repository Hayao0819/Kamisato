package shared

import (
	"github.com/Hayao0819/Kamisato/internal/blinkyutils"
)

// AyatoServer is a resolved ayato endpoint: base URL plus credentials from the blinky server database.
type AyatoServer = blinkyutils.ServerInfo

// ResolveAyatoServer looks up the base URL and credentials in the serverdb,
// using the default server when server is empty. This is the same store blinky
// uploads use, so a server from `ayaka server add` works here too.
func ResolveAyatoServer(server string) (*AyatoServer, error) {
	return blinkyutils.ResolveServer(server)
}
