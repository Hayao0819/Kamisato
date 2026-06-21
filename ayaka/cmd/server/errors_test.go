package servercmd

import (
	stderrors "errors"
	"testing"

	"github.com/Hayao0819/Kamisato/internal/utils"
)

// TestErrServerNotFoundMatchesThroughWrap confirms the server-registry sentinel
// stays matchable with errors.Is after utils.WrapErr adds the server name.
func TestErrServerNotFoundMatchesThroughWrap(t *testing.T) {
	wrapped := utils.WrapErr(ErrServerNotFound, "example.com")
	if !stderrors.Is(wrapped, ErrServerNotFound) {
		t.Fatal("errors.Is(wrapped, ErrServerNotFound) = false; want true")
	}
}
