package blinkyutils

import (
	"sort"
	"strings"

	blinky_util "github.com/BrenekH/blinky/cmd/blinky/util"
	"github.com/Hayao0819/Kamisato/internal/errwrap"
)

// Server is one stored ayato endpoint credential pair. Aliased so callers depend
// on blinkyutils rather than importing blinky's util package directly.
type Server = blinky_util.Server

// ServerDB is the on-disk registry of ayato endpoints plus the default selection,
// shared by blinky uploads and ayaka's own server commands.
type ServerDB = blinky_util.ServerDB

// ReadServerDB loads the shared server registry.
func ReadServerDB() (ServerDB, error) {
	db, err := blinky_util.ReadServerDB()
	if err != nil {
		return db, errwrap.WrapErr(err, "failed to read server database")
	}
	return db, nil
}

// SaveServerDB persists the shared server registry.
func SaveServerDB(db ServerDB) error {
	return errwrap.WrapErr(blinky_util.SaveServerDB(db), "failed to save server database")
}

// ServerNames returns the registered server names starting with prefix, sorted.
// It is the single source for the server-name shell completions, so each command
// no longer reimplements the same db scan.
func ServerNames(prefix string) []string {
	db, err := blinky_util.ReadServerDB()
	if err != nil {
		return nil
	}
	names := make([]string, 0, len(db.Servers))
	for name := range db.Servers {
		if strings.HasPrefix(name, prefix) {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}
