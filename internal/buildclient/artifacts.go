package buildclient

import (
	"context"
	"io"
)

func ListArtifacts(ctx context.Context, base, token, id string) ([]string, error) {
	c, err := ayato(base, token)
	if err != nil {
		return nil, err
	}
	return c.ListArtifacts(ctx, id)
}

func DownloadArtifact(ctx context.Context, base, token, id, name string, writer io.Writer) error {
	c, err := ayato(base, token)
	if err != nil {
		return err
	}
	return c.DownloadArtifact(ctx, id, name, writer)
}
