package nvcheck

import "net/http"

// WithGitHubToken wraps base so requests to api.github.com carry an
// Authorization header, lifting CI off the unauthenticated rate limit.
func WithGitHubToken(base *http.Client, token string) *http.Client {
	if token == "" {
		return base
	}
	c := *base
	c.Transport = &githubAuthTransport{token: token, base: base.Transport}
	return &c
}

type githubAuthTransport struct {
	token string
	base  http.RoundTripper
}

func (t *githubAuthTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.URL.Host == "api.github.com" {
		req = req.Clone(req.Context())
		req.Header.Set("Authorization", "Bearer "+t.token)
	}
	rt := t.base
	if rt == nil {
		rt = http.DefaultTransport
	}
	return rt.RoundTrip(req)
}
