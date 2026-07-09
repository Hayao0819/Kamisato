package shared

import (
	"github.com/Hayao0819/Kamisato/internal/blinkyutils"
	"github.com/Hayao0819/Kamisato/internal/errors"
)

// Sentinel errors for the ayaka command layer. Package-level so callers can
// errors.Is them through errors.WrapErr; don't build inline, since two NewErr
// values with the same message are distinct.
var (
	ErrInvalidRepoName    = errors.NewErr("invalid repository name")
	ErrSourceRepoNotFound = errors.NewErr("source repository not found")
	ErrNoSourceDir        = errors.NewErr("source directory not found")
	ErrNoDestDir          = errors.NewErr("destination directory not found")
	// Server resolution lives in blinkyutils; alias its sentinels so the command
	// layer keeps a single shared.ErrServerNotFound value to errors.Is against.
	ErrServerNotFound    = blinkyutils.ErrServerNotFound
	ErrNoServerSpecified = blinkyutils.ErrNoServerSpecified
)
