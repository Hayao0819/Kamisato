// Package errutil contains small error helpers shared by backend implementations.
package errutil

import (
	"context"
	"errors"
	"fmt"
	"os/exec"

	"github.com/Hayao0819/Kamisato/pkg/pacman/builder"
)

func Wrap(err error, message string) error {
	return fmt.Errorf("%s: %w", message, err)
}

// BuildFailure marks a non-zero build-phase exit as deterministic while
// preserving cancellation and deadline errors as transient/caller-controlled.
func BuildFailure(ctx context.Context, err error, message string) error {
	if ctxErr := ctx.Err(); ctxErr != nil {
		return fmt.Errorf("%s: %w", message, ctxErr)
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		return Wrap(err, message)
	}
	return fmt.Errorf("%w: %s: %w", builder.ErrBuildFailed, message, err)
}
