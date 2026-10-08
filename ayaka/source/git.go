package source

import (
	"slices"
	"strings"
)

type GitSource struct {
	Remote string
	Ref    string
}

func ParseGitSource(value string) (GitSource, bool) {
	if _, source, ok := strings.Cut(value, "::"); ok {
		value = source
	}

	scheme, rest, ok := strings.Cut(value, "://")
	if !ok {
		return GitSource{}, false
	}
	protocols := strings.Split(scheme, "+")
	if !slices.Contains(protocols, "git") {
		return GitSource{}, false
	}

	location, fragment, hasFragment := strings.Cut(rest, "#")
	location, _, _ = strings.Cut(location, "?")
	if location == "" {
		return GitSource{}, false
	}

	ref := "HEAD"
	if hasFragment {
		fragment, _, _ = strings.Cut(fragment, "?")
		kind, value, ok := strings.Cut(fragment, "=")
		if !ok || value == "" || kind != "branch" {
			return GitSource{}, false
		}
		ref = value
	}

	return GitSource{
		Remote: protocols[len(protocols)-1] + "://" + location,
		Ref:    ref,
	}, true
}
