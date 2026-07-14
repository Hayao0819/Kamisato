package auth

import (
	"crypto/subtle"

	"github.com/Hayao0819/Kamisato/internal/conf"
)

type apiKeyAuth struct {
	keys []apiKeyEntry
}

type apiKeyEntry struct {
	name  string
	key   []byte
	repos map[string]bool
}

func newAPIKeyAuth(cfg []conf.CIAPIKey) *apiKeyAuth {
	a := &apiKeyAuth{}
	for _, k := range cfg {
		e := apiKeyEntry{name: k.Name, key: []byte(k.Key), repos: map[string]bool{}}
		for _, r := range k.PublishRepos {
			e.repos[r] = true
		}
		a.keys = append(a.keys, e)
	}
	return a
}

// authorize constant-time-compares the presented key against every key with no
// early return, so timing doesn't reveal which matched, then checks its repo scope.
func (a *apiKeyAuth) authorize(presented, repo string) (*CIPrincipal, bool) {
	p := []byte(presented)
	matched := -1
	for i := range a.keys {
		if subtle.ConstantTimeCompare(p, a.keys[i].key) == 1 {
			matched = i
		}
	}
	if matched < 0 {
		return nil, false
	}
	e := a.keys[matched]
	if !e.repos[repo] && !e.repos["*"] {
		return nil, false
	}
	return &CIPrincipal{Via: "apikey", ID: e.name}, true
}
