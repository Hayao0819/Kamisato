package nvcheck

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/Hayao0819/Kamisato/pkg/pacman/alpm"
)

// Entry is one monitored package: how to find its latest upstream version and how
// to rebuild it when a newer one appears.
type Entry struct {
	Pkgbase string
	Source  Spec
	// Build target carried through to the enqueued rebuild.
	Repo string
	Arch string
	Git  string
}

// Enqueuer starts a rebuild for a monitored entry whose upstream moved to
// newVersion. The service's real enqueuer tags the job ReasonVersionUpdate.
type Enqueuer interface {
	EnqueueVersionUpdate(entry Entry, newVersion string) error
}

// CurrentFunc reports the version currently built/published for an entry. An
// empty result means "unknown" (treated as out-of-date so the first check
// publishes a baseline).
type CurrentFunc func(ctx context.Context, entry Entry) (string, error)

// Result records the outcome of checking one entry.
type Result struct {
	Pkgbase  string
	Latest   string
	Current  string
	Outdated bool
	Enqueued bool
	Err      error
}

// Checker runs a version-check pass over a set of entries.
type Checker struct {
	entries []Entry
	client  *http.Client
	current CurrentFunc
	enq     Enqueuer
	log     *slog.Logger
}

// CheckerOptions names the collaborators used by a check pass. Enqueuer is
// optional: leaving it nil makes the checker read-only.
type CheckerOptions struct {
	HTTPClient     *http.Client
	CurrentVersion CurrentFunc
	Enqueuer       Enqueuer
	Logger         *slog.Logger
}

// NewChecker builds a Checker. A nil enqueuer makes the pass a dry run (it
// reports outdated entries without triggering a rebuild); a nil logger discards.
func NewChecker(entries []Entry, options CheckerOptions) *Checker {
	if options.Logger == nil {
		options.Logger = slog.New(slog.DiscardHandler)
	}
	if options.CurrentVersion == nil {
		options.CurrentVersion = func(context.Context, Entry) (string, error) {
			return "", fmt.Errorf("nvcheck: current version resolver is not configured")
		}
	}
	return &Checker{
		entries: entries,
		client:  options.HTTPClient,
		current: options.CurrentVersion,
		enq:     options.Enqueuer,
		log:     options.Logger,
	}
}

// Check runs one pass over every entry, returning a Result per entry. A per-entry
// error (fetch, compare, enqueue) is captured in its Result and never aborts the
// pass, so one unreachable upstream does not stall the rest.
func (c *Checker) Check(ctx context.Context) []Result {
	results := make([]Result, 0, len(c.entries))
	for _, e := range c.entries {
		results = append(results, c.checkOne(ctx, e))
	}
	return results
}

func (c *Checker) checkOne(ctx context.Context, e Entry) Result {
	res := Result{Pkgbase: e.Pkgbase}

	src, err := NewSource(e.Source, c.client)
	if err != nil {
		res.Err = err
		return res
	}
	latest, err := src.Latest(ctx)
	if err != nil {
		res.Err = fmt.Errorf("fetch latest for %s: %w", e.Pkgbase, err)
		return res
	}
	res.Latest = latest

	current, err := c.current(ctx, e)
	if err != nil {
		res.Err = fmt.Errorf("current version for %s: %w", e.Pkgbase, err)
		return res
	}
	res.Current = current

	res.Outdated = isNewer(latest, current)
	if !res.Outdated {
		return res
	}

	c.log.Info("upstream version is newer", "pkgbase", e.Pkgbase, "current", current, "latest", latest)
	if c.enq == nil {
		return res
	}
	if err := c.enq.EnqueueVersionUpdate(e, latest); err != nil {
		res.Err = fmt.Errorf("enqueue rebuild for %s: %w", e.Pkgbase, err)
		return res
	}
	res.Enqueued = true
	return res
}

// isNewer treats an unknown (empty) current as older, so the first observation
// establishes a baseline rather than being silently skipped.
func isNewer(latest, current string) bool {
	if latest == "" {
		return false
	}
	if current == "" {
		return true
	}
	cmp, _ := alpm.VerCmp(latest, current)
	return cmp > 0
}
