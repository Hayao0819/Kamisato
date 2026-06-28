package servercmd

import "github.com/Hayao0819/Kamisato/internal/utils"

// ErrServerNotFound is a package-level sentinel so callers can errors.Is it through utils.WrapErr.
var ErrServerNotFound = utils.NewErr("server not found")
