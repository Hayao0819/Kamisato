package shared

import (
	"fmt"

	blinky_util "github.com/BrenekH/blinky/cmd/blinky/util"
	"github.com/Hayao0819/Kamisato/internal/utils"
)

// AyatoServer is a resolved ayato endpoint: the base URL plus the Basic-auth
// credentials stored for it in the blinky server database.
type AyatoServer struct {
	URL      string
	Username string
	Password string
}

// ResolveAyatoServer looks up the ayato base URL and credentials from the same
// serverdb the other commands use. When server is empty the database's default
// server is used. The same store backs blinky uploads, so a server registered
// with `ayaka server add` works here too.
func ResolveAyatoServer(server string) (*AyatoServer, error) {
	db, err := blinky_util.ReadServerDB()
	if err != nil {
		return nil, utils.WrapErr(err, "failed to read server database")
	}

	if server == "" {
		server = db.DefaultServer
	}
	if server == "" {
		return nil, ErrNoServerSpecified
	}

	entry, ok := db.Servers[server]
	if !ok {
		return nil, utils.WrapErr(ErrServerNotFound, fmt.Sprintf(
			"server %q is not registered; log in first with 'ayaka server login %s'",
			server, server))
	}

	return &AyatoServer{
		URL:      server,
		Username: entry.Username,
		Password: entry.Password,
	}, nil
}
