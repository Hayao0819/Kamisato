package shared

import (
	"fmt"

	blinky_util "github.com/BrenekH/blinky/cmd/blinky/util"
	"github.com/Hayao0819/Kamisato/internal/utils"
)

// AyatoServer is a resolved ayato endpoint: base URL plus credentials from the blinky server database.
type AyatoServer struct {
	URL      string
	Username string
	Password string
}

// ResolveAyatoServer looks up the base URL and credentials in the serverdb,
// using the default server when server is empty. This is the same store blinky
// uploads use, so a server from `ayaka server add` works here too.
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
