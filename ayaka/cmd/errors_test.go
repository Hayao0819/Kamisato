package cmd

import (
	stderrors "errors"
	"testing"

	"github.com/Hayao0819/Kamisato/internal/utils"
)

// TestSentinelErrorsMatchThroughWrap confirms every command-layer sentinel
// stays matchable with errors.Is after utils.WrapErr adds context, including
// through more than one layer of wrapping.
func TestSentinelErrorsMatchThroughWrap(t *testing.T) {
	sentinels := map[string]error{
		"ErrInvalidRepoName":    ErrInvalidRepoName,
		"ErrSourceRepoNotFound": ErrSourceRepoNotFound,
		"ErrNoSourceDir":        ErrNoSourceDir,
		"ErrNoDestDir":          ErrNoDestDir,
		"ErrServerNotFound":     ErrServerNotFound,
		"ErrNoServerSpecified":  ErrNoServerSpecified,
	}

	for name, sentinel := range sentinels {
		wrapped := utils.WrapErr(sentinel, "context")
		if !stderrors.Is(wrapped, sentinel) {
			t.Errorf("%s: errors.Is failed through one WrapErr", name)
		}
		double := utils.WrapErr(wrapped, "outer context")
		if !stderrors.Is(double, sentinel) {
			t.Errorf("%s: errors.Is failed through two WrapErr layers", name)
		}
	}
}

// TestSentinelErrorsAreDistinct guards against accidentally aliasing two
// sentinels, and confirms a wrapped sentinel does not match a different one.
func TestSentinelErrorsAreDistinct(t *testing.T) {
	if stderrors.Is(ErrServerNotFound, ErrNoServerSpecified) {
		t.Error("ErrServerNotFound and ErrNoServerSpecified compare equal")
	}
	if stderrors.Is(utils.WrapErr(ErrInvalidRepoName, "myrepo"), ErrSourceRepoNotFound) {
		t.Error("wrapped ErrInvalidRepoName matched ErrSourceRepoNotFound")
	}
}
