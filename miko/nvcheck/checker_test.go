package nvcheck

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

type fakeEnqueuer struct {
	calls []struct {
		pkgbase string
		version string
	}
}

func (f *fakeEnqueuer) EnqueueVersionUpdate(entry Entry, newVersion string) error {
	f.calls = append(f.calls, struct {
		pkgbase string
		version string
	}{entry.Pkgbase, newVersion})
	return nil
}

// latestServer serves a fixed upstream version for the http source kind.
func latestServer(t *testing.T, version string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("version=" + version + "\n"))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func httpEntry(pkgbase, srvURL string) Entry {
	return Entry{
		Pkgbase: pkgbase,
		Source:  Spec{Kind: "http", URL: srvURL, Regex: `version=([0-9.]+)`},
		Repo:    "extra",
		Arch:    "x86_64",
	}
}

func TestCheckEnqueuesWhenUpstreamNewer(t *testing.T) {
	srv := latestServer(t, "2.0.0")
	enq := &fakeEnqueuer{}
	current := func(ctx context.Context, e Entry) (string, error) { return "1.0.0", nil }

	c := NewChecker([]Entry{httpEntry("foo", srv.URL)}, srv.Client(), current, enq, nil)
	results := c.Check(context.Background())

	if len(results) != 1 || !results[0].Outdated || !results[0].Enqueued {
		t.Fatalf("expected an outdated+enqueued result, got %+v", results)
	}
	if len(enq.calls) != 1 || enq.calls[0].pkgbase != "foo" || enq.calls[0].version != "2.0.0" {
		t.Errorf("enqueuer calls = %+v, want one foo@2.0.0", enq.calls)
	}
}

func TestCheckNoEnqueueWhenSameOrOlder(t *testing.T) {
	for _, current := range []string{"2.0.0", "3.0.0"} {
		srv := latestServer(t, "2.0.0")
		enq := &fakeEnqueuer{}
		cur := func(ctx context.Context, e Entry) (string, error) { return current, nil }

		c := NewChecker([]Entry{httpEntry("foo", srv.URL)}, srv.Client(), cur, enq, nil)
		results := c.Check(context.Background())

		if results[0].Outdated {
			t.Errorf("current=%s: marked outdated against upstream 2.0.0", current)
		}
		if len(enq.calls) != 0 {
			t.Errorf("current=%s: enqueued %d builds, want 0", current, len(enq.calls))
		}
	}
}

func TestCheckUnknownCurrentTreatedAsOutdated(t *testing.T) {
	srv := latestServer(t, "1.0.0")
	enq := &fakeEnqueuer{}
	current := func(ctx context.Context, e Entry) (string, error) { return "", nil }

	c := NewChecker([]Entry{httpEntry("foo", srv.URL)}, srv.Client(), current, enq, nil)
	results := c.Check(context.Background())

	if !results[0].Enqueued || len(enq.calls) != 1 {
		t.Errorf("an unknown current version should establish a baseline build, got %+v", results)
	}
}

func TestCheckPerEntryErrorDoesNotAbortPass(t *testing.T) {
	good := latestServer(t, "2.0.0")
	enq := &fakeEnqueuer{}
	current := func(ctx context.Context, e Entry) (string, error) { return "1.0.0", nil }

	entries := []Entry{
		{Pkgbase: "bad", Source: Spec{Kind: "bogus"}},
		httpEntry("good", good.URL),
	}
	c := NewChecker(entries, good.Client(), current, enq, nil)
	results := c.Check(context.Background())

	if len(results) != 2 {
		t.Fatalf("want 2 results, got %d", len(results))
	}
	if results[0].Err == nil {
		t.Error("bad entry should carry an error")
	}
	if !results[1].Enqueued {
		t.Error("a later healthy entry should still be checked and enqueued")
	}
}

func TestCheckDryRunReportsWithoutEnqueue(t *testing.T) {
	srv := latestServer(t, "2.0.0")
	current := func(ctx context.Context, e Entry) (string, error) { return "1.0.0", nil }

	// A nil enqueuer makes the pass a dry run.
	c := NewChecker([]Entry{httpEntry("foo", srv.URL)}, srv.Client(), current, nil, nil)
	results := c.Check(context.Background())

	if !results[0].Outdated || results[0].Enqueued {
		t.Errorf("dry run should report outdated but not enqueue, got %+v", results[0])
	}
}
