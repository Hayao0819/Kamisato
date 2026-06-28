package shared

import "github.com/Hayao0819/Kamisato/internal/utils"

// Sentinel errors for the ayaka command layer. Package-level so callers can
// errors.Is them through utils.WrapErr; don't build inline, since two NewErr
// values with the same message are distinct.
var (
	ErrInvalidRepoName    = utils.NewErr("invalid repository name")
	ErrSourceRepoNotFound = utils.NewErr("source repository not found")
	ErrNoSourceDir        = utils.NewErr("source directory not found")
	ErrNoDestDir          = utils.NewErr("destination directory not found")
	ErrServerNotFound     = utils.NewErr("server not found")
	ErrNoServerSpecified  = utils.NewErr("no server specified and no default server is set")
)
