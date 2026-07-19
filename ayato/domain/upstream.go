package domain

import (
	"fmt"
	"net/url"
	"strings"
)

type UpstreamSpec struct {
	DBURL    string
	FilesURL string
}

// Upstream is a validated remote database overlay definition.
type Upstream struct {
	dbURL    string
	filesURL string
}

func newUpstream(spec UpstreamSpec) (Upstream, error) {
	if spec.DBURL == "" {
		if spec.FilesURL != "" {
			return Upstream{}, fmt.Errorf("upstream files_url requires db_url")
		}
		return Upstream{}, nil
	}
	if err := validateUpstreamURL("db_url", spec.DBURL); err != nil {
		return Upstream{}, err
	}
	if spec.FilesURL != "" {
		if err := validateUpstreamURL("files_url", spec.FilesURL); err != nil {
			return Upstream{}, err
		}
	}
	return Upstream{
		dbURL:    spec.DBURL,
		filesURL: spec.FilesURL,
	}, nil
}

func validateUpstreamURL(field, value string) error {
	expanded := strings.ReplaceAll(value, "$arch", "x86_64")
	parsed, err := url.Parse(expanded)
	if err != nil ||
		(parsed.Scheme != "http" && parsed.Scheme != "https") ||
		parsed.Host == "" {
		return fmt.Errorf(
			"upstream %s must be an absolute http(s) URL with a host: %q",
			field,
			value,
		)
	}
	return nil
}

func (u Upstream) Enabled() bool {
	return u.dbURL != ""
}

func (u Upstream) DBURLFor(arch string) string {
	return strings.ReplaceAll(u.dbURL, "$arch", arch)
}

func (u Upstream) FilesURLFor(arch string) string {
	filesURL := u.filesURL
	if filesURL == "" {
		filesURL = deriveFilesURL(u.dbURL)
	}
	return strings.ReplaceAll(filesURL, "$arch", arch)
}

func deriveFilesURL(dbURL string) string {
	index := strings.LastIndex(dbURL, ".db")
	if index < 0 {
		return dbURL
	}
	return dbURL[:index] + ".files" + dbURL[index+len(".db"):]
}
