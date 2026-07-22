package shared

import (
	"testing"

	"github.com/Hayao0819/Kamisato/internal/errors"
)

// TestSentinelErrorsMatchThroughWrap confirms sentinels stay errors.Is-matchable through nested WrapErr.
func TestSentinelErrorsMatchThroughWrap(t *testing.T) {
	sentinels := map[string]error{
		"ErrSourceRepoNotFound": ErrSourceRepoNotFound,
		"ErrNoSourceDir":        ErrNoSourceDir,
		"ErrNoDestDir":          ErrNoDestDir,
		"ErrServerNotFound":     ErrServerNotFound,
		"ErrNoServerSpecified":  ErrNoServerSpecified,
	}

	for name, sentinel := range sentinels {
		wrapped := errors.WrapErr(sentinel, "context")
		if !errors.Is(wrapped, sentinel) {
			t.Errorf("%s: errors.Is failed through one WrapErr", name)
		}
		double := errors.WrapErr(wrapped, "outer context")
		if !errors.Is(double, sentinel) {
			t.Errorf("%s: errors.Is failed through two WrapErr layers", name)
		}
	}
}

// TestSentinelErrorsAreDistinct guards against aliasing distinct sentinels.
func TestSentinelErrorsAreDistinct(t *testing.T) {
	if errors.Is(ErrServerNotFound, ErrNoServerSpecified) {
		t.Error("ErrServerNotFound and ErrNoServerSpecified compare equal")
	}
	if errors.Is(errors.WrapErr(ErrNoSourceDir, "myrepo"), ErrSourceRepoNotFound) {
		t.Error("wrapped ErrNoSourceDir matched ErrSourceRepoNotFound")
	}
}
