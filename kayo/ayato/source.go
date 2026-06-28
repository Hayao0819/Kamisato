// Package ayato makes a remote ayato instance act as a kayo package source. It
// fetches the instance's catalog (its own-hosted PKGBUILDs plus their git URLs)
// and implements aurweb.Backend so kayo can federate ayato alongside local git
// overlays and the upstream AUR.
package ayato

import (
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/Hayao0819/Kamisato/pkg/aurweb"
)

const (
	catalogPath     = "/api/unstable/aur/catalog"
	pubkeyPath      = "/api/unstable/aur/pubkey"
	maxCatalogBytes = 32 << 20
)

// Options configures a Source.
type Options struct {
	Name, BaseURL string
	PubKey        string        // explicit hard pin (base64); "" => TOFU or Insecure
	MaxAge        time.Duration // freshness ceiling; must be > 0 unless Insecure
	Insecure      bool          // accept an unsigned catalog (escape hatch)
	Tofu          bool          // allow trust-on-first-use when PubKey is empty
	Pins          *PinStore
}

// Source is one ayato instance, refreshed by Sync. A catalog only swaps in after
// its signature and freshness verify (unless Insecure), so the index never holds
// unverified data.
type Source struct {
	name     string
	base     string
	client   *http.Client
	maxAge   time.Duration
	insecure bool
	tofu     bool
	pins     *PinStore

	mu           sync.RWMutex
	index        map[string]aurweb.Pkg
	sources      map[string]string
	names        []string
	verifier     *Verifier // set when explicitly pinned
	lastIssued   time.Time // anti-rollback watermark (in-memory)
	expiresAt    time.Time // served catalog's signed expiry (zero if none)
	lastVerified bool      // last Sync passed signature+freshness
}

// New builds a Source. An explicit PubKey is a hard pin; an empty PubKey requires
// either Tofu or Insecure.
func New(o Options) (*Source, error) {
	s := &Source{
		name:     o.Name,
		base:     strings.TrimRight(o.BaseURL, "/"),
		client:   &http.Client{Timeout: 15 * time.Second},
		maxAge:   o.MaxAge,
		insecure: o.Insecure,
		tofu:     o.Tofu,
		pins:     o.Pins,
		index:    map[string]aurweb.Pkg{},
		sources:  map[string]string{},
	}
	if o.PubKey != "" {
		v, err := NewVerifier(o.PubKey, o.MaxAge)
		if err != nil {
			return nil, err
		}
		s.verifier = v
	}
	if o.Pins != nil { // restore the rollback watermark across restarts
		if p, ok := o.Pins.Get(o.Name); ok {
			s.lastIssued = p.LastIssued
		}
	}
	return s, nil
}
