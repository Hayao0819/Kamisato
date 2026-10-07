package cli

import (
	ayatostore "github.com/Hayao0819/Kamisato/internal/api/ayato/auth/store"
	"github.com/Hayao0819/Kamisato/internal/errors"
)

// Sentinel errors for the ayaka command layer. Package-level so callers can
// errors.Is them through errors.WrapErr; don't build inline, since two NewErr
// values with the same message are distinct.
var (
	ErrSourceRepoNotFound = errors.NewErr("source repository not found")
	ErrNoSourceDir        = errors.NewErr("source directory not found")
	ErrNoDestDir          = errors.NewErr("destination directory not found")
	ErrServerNotFound     = ayatostore.ErrServerNotFound
	ErrNoServerSpecified  = ayatostore.ErrNoServerSpecified
)
