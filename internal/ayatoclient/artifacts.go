package ayatoclient

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"

	"github.com/Hayao0819/Kamisato/internal/utils"
)

// ListArtifacts returns the downloadable artifact names of a client-signed job.
func ListArtifacts(ctx context.Context, base, id string) ([]string, error) {
	resp, err := get(ctx, apiClient, endpoint(base, "/api/unstable/jobs/"+id+"/artifacts"))
	if err != nil {
		return nil, utils.WrapErr(err, "failed to list artifacts")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, responseErr(resp, "list artifacts")
	}
	var out struct {
		Artifacts []string `json:"artifacts"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, utils.WrapErr(err, "failed to decode artifacts")
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
