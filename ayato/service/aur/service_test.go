package aur

import (
	"context"
	"testing"
	"time"

	"github.com/Hayao0819/Kamisato/internal/kayoproto"
)

// countingSM is a SourceManager whose Catalog build is counted, so a test can prove
// the service serves a cached envelope instead of rebuilding.
type countingSM struct {
	calls int
}

func (c *countingSM) Register(context.Context, string, string, string) (string, []string, error) {
	return "", nil, nil
}
func (c *countingSM) Remove(context.Context, string) error   { return nil }
func (c *countingSM) List(context.Context) ([]string, error) { return nil, nil }
func (c *countingSM) Catalog(context.Context) (kayoproto.Catalog, error) {
	c.calls++
	return kayoproto.Catalog{}, nil
}

func TestCatalogServiceCachesEnvelope(t *testing.T) {
	sm := &countingSM{}
	svc := NewService(sm, time.Minute)

	if _, err := svc.Envelope(context.Background()); err != nil {
		t.Fatalf("Envelope: %v", err)
	}
	if sm.calls != 1 {
		t.Fatalf("first build made %d catalog builds, want 1", sm.calls)
	}
	if _, err := svc.Envelope(context.Background()); err != nil {
		t.Fatalf("Envelope: %v", err)
	}
	if sm.calls != 1 {
		t.Fatalf("cached catalog rebuilt: %d catalog builds after second hit, want 1", sm.calls)
	}
}
