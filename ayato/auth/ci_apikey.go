package auth

import (
	"crypto/subtle"

	"github.com/Hayao0819/Kamisato/internal/conf"
)

type apiKeyAuth struct {
	keys []apiKeyEntry
}

type apiKeyEntry struct {
	name   string
	key    []byte
	repos  map[string]bool
	scopes map[string]bool
}

type apiKeyGrant uint8

const (
	repositoryGrant apiKeyGrant = iota
	serviceGrant
)

func newAPIKeyAuth(cfg []conf.CIAPIKey) *apiKeyAuth {
	a := &apiKeyAuth{}
	for _, k := range cfg {
		e := apiKeyEntry{name: k.Name, key: []byte(k.Key), repos: map[string]bool{}, scopes: map[string]bool{}}
		for _, r := range k.PublishRepos {
			e.repos[r] = true
		}
		for _, scope := range k.Scopes {
			e.scopes[scope] = true
		}
		a.keys = append(a.keys, e)
	}
	return a
}

func (a *apiKeyAuth) authorizeScope(presented, scope string) (*CIPrincipal, bool) {
	return a.authorizeGrant(presented, scope, serviceGrant)
}

func (a *apiKeyAuth) match(presented string) (apiKeyEntry, bool) {
	p := []byte(presented)
	matched := -1
	for i := range a.keys {
		if subtle.ConstantTimeCompare(p, a.keys[i].key) == 1 {
			matched = i
		}
	}
	if matched < 0 {
		return apiKeyEntry{}, false
	}
	return a.keys[matched], true
}

// authorize checks the key and repository scope without early secret-dependent exits.
func (a *apiKeyAuth) authorize(presented, repo string) (*CIPrincipal, bool) {
	return a.authorizeGrant(presented, repo, repositoryGrant)
}

func (a *apiKeyAuth) authorizeGrant(
	presented string,
	target string,
	grant apiKeyGrant,
) (*CIPrincipal, bool) {
	entry, ok := a.match(presented)
	if !ok {
		return nil, false
	}
	principal := &CIPrincipal{Via: "apikey", ID: entry.name}
	allowed := entry.repos
	if grant == serviceGrant {
		allowed = entry.scopes
	}
	if !allowed[target] && !allowed["*"] {
		return principal, false
	}
	return principal, true
}
