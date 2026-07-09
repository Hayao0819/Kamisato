package service

import (
	"context"
	"fmt"
	"testing"

	"github.com/Hayao0819/Kamisato/internal/errors"

	"github.com/Hayao0819/Kamisato/pkg/pacman/builder"
)

func TestIsRetriable(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil is not retriable", nil, false},
		{"compile failure is terminal", builder.ErrBuildFailed, false},
		{
			"wrapped compile failure is terminal",
			errors.WrapErr(builder.ErrBuildFailed, "dependency build failed"),
			false,
		},
		{
			"fmt-wrapped compile failure is terminal",
			// Mirrors how the container backend reports a non-zero exit code.
			fmt.Errorf("%w with exit code 1", builder.ErrBuildFailed),
			false,
		},
		{"clone failure is transient", errors.New("failed to clone AUR dependency"), true},
		{"build timeout is transient", context.DeadlineExceeded, true},
		{"image pull failure is transient", errors.New("failed to pull image"), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isRetriable(tt.err); got != tt.want {
				t.Errorf("isRetriable(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}
