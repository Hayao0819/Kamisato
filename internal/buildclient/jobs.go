package buildclient

import (
	"context"
	"io"
)

func SubmitBuild(ctx context.Context, base, token string, request *BuildRequest) (string, error) {
	c, err := ayato(base, token)
	if err != nil {
		return "", err
	}
	return c.SubmitBuild(ctx, request)
}

func ListJobs(ctx context.Context, base, token string) ([]Job, error) {
	c, err := ayato(base, token)
	if err != nil {
		return nil, err
	}
	return c.ListJobs(ctx)
}

func JobStatus(ctx context.Context, base, token, id string) (*Job, error) {
	c, err := ayato(base, token)
	if err != nil {
		return nil, err
	}
	return c.JobStatus(ctx, id)
}

func WaitJob(ctx context.Context, base, token, id string, logs io.Writer) (*Job, error) {
	c, err := ayato(base, token)
	if err != nil {
		return nil, err
	}
	return c.WaitJob(ctx, id, logs)
}

func CancelJob(ctx context.Context, base, token, id string) error {
	c, err := ayato(base, token)
	if err != nil {
		return err
	}
	return c.CancelJob(ctx, id)
}

func FetchStats(ctx context.Context, base, token string) (*Stats, error) {
	c, err := ayato(base, token)
	if err != nil {
		return nil, err
	}
	return c.FetchStats(ctx)
}

func StreamLogs(ctx context.Context, base, token, id string, writer io.Writer) error {
	c, err := ayato(base, token)
	if err != nil {
		return err
	}
	return c.StreamLogs(ctx, id, writer)
}
