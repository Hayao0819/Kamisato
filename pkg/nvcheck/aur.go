package nvcheck

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// aurSource reads a package's current AUR version from the RPC v5 info
// endpoint, like nvchecker's aur source. The version includes the pkgrel, so a
// mirror is outdated exactly when the AUR maintainer pushed a bump.
type aurSource struct {
	pkg    string
	base   string
	client *http.Client
}

func (s *aurSource) Latest(ctx context.Context) (string, error) {
	u := strings.TrimRight(s.base, "/") + "/rpc/v5/info?arg[]=" + url.QueryEscape(s.pkg)
	var body struct {
		Results []struct {
			Version string `json:"Version"`
		} `json:"results"`
	}
	if err := fetchJSON(ctx, s.client, u, &body); err != nil {
		return "", err
	}
	if len(body.Results) == 0 || body.Results[0].Version == "" {
		return "", fmt.Errorf("nvcheck: AUR has no package %s", s.pkg)
	}
	return body.Results[0].Version, nil
}
