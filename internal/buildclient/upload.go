package buildclient

import "context"

func UploadPackageFiles(ctx context.Context, base, token, repo string, files ...string) error {
	c, err := ayato(base, token)
	if err != nil {
		return err
	}
	return c.UploadPackageFiles(ctx, repo, files...)
}
