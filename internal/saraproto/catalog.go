// Package saraproto defines the private sara<->ayato exchange types. It is
// internal on purpose: unlike pkg/aurweb (a reusable aurweb-compatible library),
// this protocol is specific to Kamisato and free to carry trust and attestation
// fields the public aurweb RPC cannot.
package saraproto

import "github.com/Hayao0819/Kamisato/pkg/aurweb"

// Catalog is an ayato instance's managed packages and the git URL each pkgbase
// is cloned from. Trust and attestation fields will be added here later.
type Catalog struct {
	Packages []aurweb.Pkg      `json:"packages"`
	Sources  map[string]string `json:"sources"` // pkgbase -> git clone URL
}
