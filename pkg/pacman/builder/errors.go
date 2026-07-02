package builder

import (
	"errors"
	"fmt"
)

// wrapErr keeps the library layer free of cockroachdb's stack-carrying wrapper;
// a plain %w is all the caller needs to unwrap.
func wrapErr(err error, msg string) error { return fmt.Errorf("%s: %w", msg, err) }

// ErrBuildFailed marks a build that ran to a deterministic failure — a non-zero
// makepkg exit (a PKGBUILD or compile error) or a build that produced no package.
// Retrying it would fail identically, so callers use errors.Is to tell it apart
// from transient failures (clone, image pull, a build timeout) worth retrying.
var ErrBuildFailed = errors.New("build failed")
