package buildclient

import "context"

func DownloadPackage(ctx context.Context, base, repo, arch, name, destination string) error {
	c, err := ayato(base, "")
	if err != nil {
		return err
	}
	return c.DownloadPackageFile(ctx, repo, arch, name, destination)
}
