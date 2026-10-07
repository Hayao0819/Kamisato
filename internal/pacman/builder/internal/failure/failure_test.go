package failure

import (
	"context"
	"errors"
	"os/exec"
	"testing"

	"github.com/Hayao0819/Kamisato/internal/pacman/builder"
)

func TestBuildFailureClassification(t *testing.T) {
	exitErr := exec.Command("sh", "-c", "exit 1").Run()
	if exitErr == nil {
		t.Fatal("test command unexpectedly succeeded")
	}
	err := Build(t.Context(), exitErr, "makepkg failed")
	if !errors.Is(err, builder.ErrBuildFailed) || !errors.Is(err, exitErr) {
		t.Fatalf("deterministic build error was not preserved/classified: %v", err)
	}

	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	err = Build(ctx, exitErr, "makepkg failed")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation was not preserved: %v", err)
	}
	if errors.Is(err, builder.ErrBuildFailed) {
		t.Fatalf("cancellation was incorrectly classified as deterministic: %v", err)
	}

	launchErr := errors.New("executable not found")
	err = Build(t.Context(), launchErr, "makepkg failed")
	if errors.Is(err, builder.ErrBuildFailed) || !errors.Is(err, launchErr) {
		t.Fatalf("launch error was misclassified: %v", err)
	}
}
