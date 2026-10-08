package builder

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
)

// ErrBuildFailed marks a deterministic failure (non-zero makepkg exit or no output); callers use errors.Is to distinguish it from transient failures worth retrying.
var ErrBuildFailed = errors.New("build failed")

// BuildError classifies a build-phase failure according to the backend contract.
// A non-zero command exit is deterministic; cancellation, deadlines, and failure
// to start a command remain transient or caller-controlled.
func BuildError(ctx context.Context, err error, message string) error {
	if ctxErr := ctx.Err(); ctxErr != nil {
		return fmt.Errorf("%s: %w", message, ctxErr)
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		return fmt.Errorf("%s: %w", message, err)
	}
	return fmt.Errorf("%w: %s: %w", ErrBuildFailed, message, err)
}
