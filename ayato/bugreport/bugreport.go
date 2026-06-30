// Package bugreport forwards user-submitted bug reports to an external tracker.
// ayato stores nothing itself: it wraps a concrete backend (GitHub first) behind
// Reporter, and an unconfigured server simply has no Reporter (reporting off).
package bugreport

import (
	"context"
	"fmt"
)

// Report is one user-submitted bug report against a package.
type Report struct {
	Pkgname     string
	Pkgver      string
	Name        string
	Email       string
	Severity    string
	Description string
}

// Reporter forwards a report to a tracker and returns a link to the created entry.
type Reporter interface {
	Report(ctx context.Context, r Report) (url string, err error)
}

// New builds the Reporter for the given backend type. An empty/none type returns
// (nil, nil): bug reporting is disabled, which callers treat as "feature off".
func New(typ, githubRepo, githubToken string) (Reporter, error) {
	switch typ {
	case "", "none", "disabled":
		return nil, nil
	case "github":
		return newGitHub(githubRepo, githubToken)
	default:
		return nil, fmt.Errorf("bugreport: unknown backend %q", typ)
	}
}
