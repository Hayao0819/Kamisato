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
	e, ok := a.match(presented)
	if !ok {
		return nil, false
	}
	principal := &CIPrincipal{Via: "apikey", ID: e.name}
	if !e.scopes[scope] && !e.scopes["*"] {
		return principal, false
	}
	return principal, true
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
	e, ok := a.match(presented)
	if !ok {
		return nil, false
	}
	principal := &CIPrincipal{Via: "apikey", ID: e.name}
	if !e.repos[repo] && !e.repos["*"] {
		return principal, false
	}
	return principal, true
}
