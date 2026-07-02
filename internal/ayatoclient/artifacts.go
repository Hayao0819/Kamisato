package ayatoclient

import (
	"context"
	"io"
	"net/http"
	"net/url"

	"github.com/Hayao0819/Kamisato/internal/errwrap"
)

// ListArtifacts returns the downloadable artifact names of a client-signed job
// (auth-gated).
func ListArtifacts(ctx context.Context, base, token, id string) ([]string, error) {
	var out struct {
		Artifacts []string `json:"artifacts"`
	}
	if err := doJSON(ctx, http.MethodGet, endpoint(base, "/api/unstable/jobs/"+id+"/artifacts"), token, nil, &out, http.StatusOK, "list artifacts"); err != nil {
		return nil, err
	}
	return out.Artifacts, nil
}

// DownloadArtifact streams one job artifact to w (auth-gated).
func DownloadArtifact(ctx context.Context, base, token, id, name string, w io.Writer) error {
	u := endpoint(base, "/api/unstable/jobs/"+id+"/artifacts/"+url.PathEscape(name))
	resp, err := get(ctx, streamClient, u, token)
	if err != nil {
		return errwrap.WrapErr(err, "failed to download artifact")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return responseErr(resp, "download artifact")
	}
	if _, err := io.Copy(w, resp.Body); err != nil {
		return errwrap.WrapErr(err, "failed to write artifact")
	}
	return nil
}
