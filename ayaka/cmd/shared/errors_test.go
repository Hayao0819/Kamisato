package shared

import (
	stderrors "errors"
	"testing"

	"github.com/Hayao0819/Kamisato/internal/errwrap"
)

// TestSentinelErrorsMatchThroughWrap confirms sentinels stay errors.Is-matchable through nested WrapErr.
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
		wrapped := errwrap.WrapErr(sentinel, "context")
		if !stderrors.Is(wrapped, sentinel) {
			t.Errorf("%s: errors.Is failed through one WrapErr", name)
		}
		double := errwrap.WrapErr(wrapped, "outer context")
		if !stderrors.Is(double, sentinel) {
			t.Errorf("%s: errors.Is failed through two WrapErr layers", name)
		}
	}
}

// TestSentinelErrorsAreDistinct guards against aliasing distinct sentinels.
func TestSentinelErrorsAreDistinct(t *testing.T) {
	if stderrors.Is(ErrServerNotFound, ErrNoServerSpecified) {
		t.Error("ErrServerNotFound and ErrNoServerSpecified compare equal")
	}
	if stderrors.Is(errwrap.WrapErr(ErrInvalidRepoName, "myrepo"), ErrSourceRepoNotFound) {
		t.Error("wrapped ErrInvalidRepoName matched ErrSourceRepoNotFound")
	}
}
