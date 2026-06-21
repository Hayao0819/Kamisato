package servercmd

import "github.com/Hayao0819/Kamisato/internal/utils"

// ErrServerNotFound is returned by the server subcommands when the named server
// is absent from the server database. It is a package-level sentinel so callers
// can match it with errors.Is, including through utils.WrapErr.
var ErrServerNotFound = utils.NewErr("server not found")
