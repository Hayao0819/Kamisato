package bugreport

import (
	"context"
	"errors"
	"log/slog"
)

// multiReporter fans a report out to every backend, counting it delivered when at
// least one accepts (a resubmit would duplicate on those that already succeeded)
// and erroring only when all fail.
type multiReporter struct {
	reporters []Reporter
}

func (m *multiReporter) Report(ctx context.Context, r Report) (string, error) {
	var firstURL string
	var errs []error
	for _, rep := range m.reporters {
		url, err := rep.Report(ctx, r)
		if err != nil {
			errs = append(errs, err)
			slog.Error("bug-report backend failed", "error", err)
			continue
		}
		if firstURL == "" && url != "" {
			firstURL = url
		}
	}
	if len(errs) == len(m.reporters) {
		return "", errors.Join(errs...)
	}
	return firstURL, nil
}
