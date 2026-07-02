package shared

import (
	"github.com/Hayao0819/Kamisato/internal/blinkyutils"
	"github.com/Hayao0819/Kamisato/internal/errwrap"
)

// Sentinel errors for the ayaka command layer. Package-level so callers can
// errors.Is them through errwrap.WrapErr; don't build inline, since two NewErr
// values with the same message are distinct.
var (
	ErrInvalidRepoName    = errwrap.NewErr("invalid repository name")
	ErrSourceRepoNotFound = errwrap.NewErr("source repository not found")
	ErrNoSourceDir        = errwrap.NewErr("source directory not found")
	ErrNoDestDir          = errwrap.NewErr("destination directory not found")
	// Server resolution lives in blinkyutils; alias its sentinels so the command
	// layer keeps a single shared.ErrServerNotFound value to errors.Is against.
	ErrServerNotFound    = blinkyutils.ErrServerNotFound
	ErrNoServerSpecified = blinkyutils.ErrNoServerSpecified
)
