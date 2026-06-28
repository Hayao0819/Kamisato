package aurweb

import (
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// maxNamesBytes bounds the decompressed packages.gz read.
const maxNamesBytes = 64 << 20

func (u *AURUpstream) DumpReader(ctx context.Context, ext bool) (io.ReadCloser, error) {
	name := "/packages-meta-ext-v1.json.gz"
	if !ext {
		name = "/packages-meta-v1.json.gz"
	}
	return u.gzipStream(ctx, u.gitBase+name)
}

func (u *AURUpstream) FetchNames(ctx context.Context) ([]string, error) {
	rc, err := u.gzipStream(ctx, u.gitBase+"/packages.gz")
	if err != nil {
		return nil, err
	}
	defer func() { _ = rc.Close() }()

	data, err := io.ReadAll(io.LimitReader(rc, maxNamesBytes))
	if err != nil {
		return nil, err
	}
	var out []string
	for _, line := range strings.Split(string(data), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			out = append(out, line)
		}
	}
	return out, nil
}

// gzipStream GETs a .gz URL and returns its decompressed body; closing the
// result closes the underlying response.
func (u *AURUpstream) gzipStream(ctx context.Context, rawURL string) (io.ReadCloser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", u.userAgent)

	resp, err := u.dumpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("aurweb: upstream dump request: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()
		return nil, fmt.Errorf("aurweb: upstream dump status %d", resp.StatusCode)
	}
	gz, err := gzip.NewReader(resp.Body)
	if err != nil {
		_ = resp.Body.Close()
		return nil, fmt.Errorf("aurweb: upstream dump gunzip: %w", err)
	}
	return gzipReadCloser{gz: gz, body: resp.Body}, nil
}

type gzipReadCloser struct {
	gz   *gzip.Reader
	body io.ReadCloser
}

func (g gzipReadCloser) Read(p []byte) (int, error) { return g.gz.Read(p) }

func (g gzipReadCloser) Close() error {
	err := g.gz.Close()
	if berr := g.body.Close(); err == nil {
		err = berr
	}
	return err
}

func (u *AURUpstream) SnapshotURL(pkgbase string) string {
	return u.gitBase + "/cgit/aur.git/snapshot/" + url.PathEscape(pkgbase) + ".tar.gz"
}

func (u *AURUpstream) PlainURL(pkgname string) string {
	return u.gitBase + "/cgit/aur.git/plain/PKGBUILD?h=" + url.QueryEscape(pkgname)
}
