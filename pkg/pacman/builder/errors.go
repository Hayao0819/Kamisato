package builder

import (
	"errors"
)

// ErrBuildFailed marks a deterministic failure (non-zero makepkg exit or no output); callers use errors.Is to distinguish it from transient failures worth retrying.
var ErrBuildFailed = errors.New("build failed")
