package shared

import "github.com/Hayao0819/Kamisato/internal/utils"

// Sentinel errors for the ayaka command layer. They are package-level values so
// callers can match them with errors.Is, including through utils.WrapErr (which
// preserves the cause chain). Do not build these inline at the call site: two
// utils.NewErr values with the same message are distinct and would not compare
// equal.
var (
	// ErrInvalidRepoName is returned when a repository argument names no known
	// source repository.
	ErrInvalidRepoName = utils.NewErr("invalid repository name")
	// ErrSourceRepoNotFound is returned when the source repository cannot be
	// resolved from configuration.
	ErrSourceRepoNotFound = utils.NewErr("source repository not found")
	// ErrNoSourceDir is returned when a repository has no source directory.
	ErrNoSourceDir = utils.NewErr("source directory not found")
	// ErrNoDestDir is returned when a repository has no destination directory.
	ErrNoDestDir = utils.NewErr("destination directory not found")
	// ErrServerNotFound is returned when the named ayato server is absent from
	// the server database.
	ErrServerNotFound = utils.NewErr("server not found")
	// ErrNoServerSpecified is returned when no server is given and no default is
	// configured.
	ErrNoServerSpecified = utils.NewErr("no server specified and no default server is set")
)
