package builder

import (
	"errors"
	"fmt"
)

// wrapErr keeps the library layer free of cockroachdb's stack-carrying wrapper;
// a plain %w is all the caller needs to unwrap.
func wrapErr(err error, msg string) error { return fmt.Errorf("%s: %w", msg, err) }

// ErrBuildFailed marks a deterministic failure (non-zero makepkg exit or no output); callers use errors.Is to distinguish it from transient failures worth retrying.
var ErrBuildFailed = errors.New("build failed")
