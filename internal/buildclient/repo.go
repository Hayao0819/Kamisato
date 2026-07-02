package buildclient

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"os"

	"github.com/Hayao0819/Kamisato/internal/errwrap"
)

// DownloadPackage fetches a built package file from ayato's public repo route
// (/repo/<repo>/<arch>/<file>) and writes it to dest. An arch=any package is
// served under any concrete arch via ayato's fallback, so requesting the build
// arch works regardless of the package's own arch.
func DownloadPackage(ctx context.Context, base, repo, arch, file, dest string) error {
	resp, err := get(ctx, streamClient, endpoint(base, "/repo/"+url.PathEscape(repo)+"/"+url.PathEscape(arch)+"/"+url.PathEscape(file)), "")
	if err != nil {
		return errwrap.WrapErr(err, "failed to download "+file)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return responseErr(resp, "download "+file)
	}

	f, err := os.Create(dest)
	if err != nil {
		return errwrap.WrapErr(err, "failed to create "+dest)
	}
	if _, err := io.Copy(f, resp.Body); err != nil {
		_ = f.Close()
		return errwrap.WrapErr(err, "failed to write "+dest)
	}
	return f.Close()
}
