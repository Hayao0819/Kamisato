package ayatoclient

import (
	"io"
	"net/http"
	"net/url"
	"os"

	"github.com/Hayao0819/Kamisato/internal/utils"
)

// DownloadPackage fetches a built package file from ayato's public repo route
// (/repo/<repo>/<arch>/<file>) and writes it to dest. An arch=any package is
// served under any concrete arch via ayato's fallback, so requesting the build
// arch works regardless of the package's own arch.
func DownloadPackage(base, repo, arch, file, dest string) error {
	resp, err := http.Get(endpoint(base, "/repo/"+url.PathEscape(repo)+"/"+url.PathEscape(arch)+"/"+url.PathEscape(file)))
	if err != nil {
		return utils.WrapErr(err, "failed to download "+file)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return responseErr(resp, "download "+file)
	}

	f, err := os.Create(dest)
	if err != nil {
		return utils.WrapErr(err, "failed to create "+dest)
	}
	if _, err := io.Copy(f, resp.Body); err != nil {
		f.Close()
		return utils.WrapErr(err, "failed to write "+dest)
	}
	return f.Close()
}
