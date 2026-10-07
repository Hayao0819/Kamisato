package nvcheck

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	alpm "github.com/Hayao0819/dyalpm"
)

// archpkgSource reads a package's version from the archlinux.org packages API,
// like nvchecker's archpkg source: testing repos are skipped and the highest
// result wins when a name exists for several repos/arches.
type archpkgSource struct {
	pkg          string
	stripRelease bool
	base         string
	client       *http.Client
}

func (s *archpkgSource) Latest(ctx context.Context) (string, error) {
	u := strings.TrimRight(s.base, "/") + "/packages/search/json/?name=" + url.QueryEscape(s.pkg)
	var body struct {
		Results []struct {
			Repo   string `json:"repo"`
			PkgVer string `json:"pkgver"`
			PkgRel string `json:"pkgrel"`
			Epoch  int    `json:"epoch"`
		} `json:"results"`
	}
	if err := fetchJSON(ctx, s.client, u, &body); err != nil {
		return "", err
	}
	// The comparison key keeps the epoch, but the result follows nvchecker's
	// archpkg output: pkgver-pkgrel, or pkgver alone with strip_release.
	bestKey, best := "", ""
	for _, r := range body.Results {
		if strings.Contains(r.Repo, "testing") || r.PkgVer == "" {
			continue
		}
		key := r.PkgVer
		if r.Epoch > 0 {
			key = strconv.Itoa(r.Epoch) + ":" + key
		}
		if r.PkgRel != "" {
			key += "-" + r.PkgRel
		}
		if bestKey != "" && alpm.VerCmp(key, bestKey) <= 0 {
			continue
		}
		bestKey = key
		best = r.PkgVer
		if !s.stripRelease && r.PkgRel != "" {
			best += "-" + r.PkgRel
		}
	}
	if best == "" {
		return "", fmt.Errorf("nvcheck: archlinux.org has no package %s", s.pkg)
	}
	return best, nil
}
