package ayatoclient

import (
	"context"
	"io"
	"net/http"
	"net/url"

	"github.com/Hayao0819/Kamisato/internal/utils"
)

// ListArtifacts returns the downloadable artifact names of a client-signed job.
func ListArtifacts(ctx context.Context, base, id string) ([]string, error) {
	var out struct {
		Artifacts []string `json:"artifacts"`
	}
	if err := doJSON(ctx, http.MethodGet, endpoint(base, "/api/unstable/jobs/"+id+"/artifacts"), "", nil, &out, http.StatusOK, "list artifacts"); err != nil {
		return nil, err
	}
	return out.Artifacts, nil
}

// DownloadArtifact streams one job artifact to w.
func DownloadArtifact(ctx context.Context, base, id, name string, w io.Writer) error {
	u := endpoint(base, "/api/unstable/jobs/"+id+"/artifacts/"+url.PathEscape(name))
	resp, err := get(ctx, streamClient, u)
	if err != nil {
		return utils.WrapErr(err, "failed to download artifact")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return responseErr(resp, "download artifact")
	}
	if _, err := io.Copy(w, resp.Body); err != nil {
		return utils.WrapErr(err, "failed to write artifact")
	}
	return nil
}
