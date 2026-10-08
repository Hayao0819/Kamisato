package client

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Hayao0819/Kamisato/internal/errors"
	"github.com/Hayao0819/Kamisato/internal/filesystem/safefile"
	httpclient "github.com/Hayao0819/Kamisato/internal/http/client"
)

func (c *Client) SubmitBuild(ctx context.Context, request *BuildRequest) (string, error) {
	var result struct {
		JobID string `json:"job_id"`
	}
	err := c.request.Execute(ctx, func() error {
		return c.request.Transport().DoJSON(
			ctx,
			httpclient.NoRetry,
			http.MethodPost,
			c.request.Transport().Endpoint("api", "unstable", "build"),
			true,
			request,
			&result,
			http.StatusAccepted,
			"build submit",
		)
	})
	if err != nil {
		return "", err
	}
	return result.JobID, nil
}

func (c *Client) ListJobs(ctx context.Context) ([]Job, error) {
	var jobs []Job
	err := c.request.Execute(ctx, func() error {
		return c.request.Transport().DoJSON(
			ctx,
			httpclient.RetryReplaySafe,
			http.MethodGet,
			c.request.Transport().Endpoint("api", "unstable", "jobs"),
			true,
			nil,
			&jobs,
			http.StatusOK,
			"list jobs",
		)
	})
	return jobs, err
}

func (c *Client) JobStatus(ctx context.Context, id string) (*Job, error) {
	var job Job
	err := c.request.Execute(ctx, func() error {
		return c.request.Transport().DoJSON(
			ctx,
			httpclient.RetryReplaySafe,
			http.MethodGet,
			c.request.Transport().Endpoint("api", "unstable", "jobs", id),
			true,
			nil,
			&job,
			http.StatusOK,
			"job status",
		)
	})
	if err != nil {
		return nil, err
	}
	return &job, nil
}

// WaitJob polls until the job is terminal.
func (c *Client) WaitJob(ctx context.Context, id string, logs io.Writer) (*Job, error) {
	streamed := false
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		job, err := c.JobStatus(ctx, id)
		if err != nil {
			return nil, err
		}
		switch job.Status {
		case "success", "failed", "cancelled":
			if logs != nil && !streamed {
				_ = c.StreamLogs(ctx, id, logs)
			}
			return job, nil
		case "running":
			if logs != nil && !streamed {
				streamed = true
				_ = c.StreamLogs(ctx, id, logs)
				continue
			}
		}

		timer := time.NewTimer(2 * time.Second)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-timer.C:
		}
	}
}

func (c *Client) CancelJob(ctx context.Context, id string) error {
	return c.request.Execute(ctx, func() error {
		return c.request.Transport().DoJSON(
			ctx,
			httpclient.NoRetry,
			http.MethodDelete,
			c.request.Transport().Endpoint("api", "unstable", "jobs", id),
			true,
			nil,
			nil,
			http.StatusOK,
			"cancel job",
		)
	})
}

func (c *Client) FetchStats(ctx context.Context) (*Stats, error) {
	var stats Stats
	err := c.request.Execute(ctx, func() error {
		return c.request.Transport().DoJSON(
			ctx,
			httpclient.RetryReplaySafe,
			http.MethodGet,
			c.request.Transport().Endpoint("api", "unstable", "stats"),
			true,
			nil,
			&stats,
			http.StatusOK,
			"stats",
		)
	})
	if err != nil {
		return nil, err
	}
	return &stats, nil
}

func (c *Client) StreamLogs(ctx context.Context, id string, writer io.Writer) error {
	return c.request.Execute(ctx, func() error {
		resp, err := c.request.Transport().Get(
			ctx,
			c.request.Transport().Endpoint("api", "unstable", "jobs", id, "logs"),
			true,
		)
		if err != nil {
			return errors.WrapErr(err, "open job log stream")
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return httpclient.ReadResponseError(resp, "job logs")
		}

		if !strings.HasPrefix(resp.Header.Get("Content-Type"), "text/event-stream") {
			if _, err := io.Copy(writer, resp.Body); err != nil {
				return errors.WrapErr(err, "read logs")
			}
			return nil
		}

		scanner := bufio.NewScanner(resp.Body)
		scanner.Buffer(make([]byte, 0, 64*1024), 1<<20)
		for scanner.Scan() {
			data, ok := strings.CutPrefix(scanner.Text(), "data:")
			if !ok {
				continue
			}
			if _, err := fmt.Fprintln(writer, strings.TrimPrefix(data, " ")); err != nil {
				return errors.WrapErr(err, "write log line")
			}
		}
		if err := scanner.Err(); err != nil {
			return errors.WrapErr(err, "read log stream")
		}
		return nil
	})
}

func (c *Client) ListArtifacts(ctx context.Context, id string) ([]string, error) {
	var result struct {
		Artifacts []string `json:"artifacts"`
	}
	err := c.request.Execute(ctx, func() error {
		return c.request.Transport().DoJSON(
			ctx,
			httpclient.RetryReplaySafe,
			http.MethodGet,
			c.request.Transport().Endpoint("api", "unstable", "jobs", id, "artifacts"),
			true,
			nil,
			&result,
			http.StatusOK,
			"list artifacts",
		)
	})
	return result.Artifacts, err
}

func (c *Client) DownloadArtifact(ctx context.Context, id, name string, writer io.Writer) error {
	return c.request.Execute(ctx, func() error {
		resp, err := c.request.Transport().Get(
			ctx,
			c.request.Transport().Endpoint("api", "unstable", "jobs", id, "artifacts", name),
			true,
		)
		if err != nil {
			return errors.WrapErr(err, "download artifact")
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return httpclient.ReadResponseError(resp, "download artifact")
		}
		if _, err := io.Copy(writer, resp.Body); err != nil {
			return errors.WrapErr(err, "write artifact")
		}
		return nil
	})
}

// DownloadPackage fetches a public repository object.
func (c *Client) DownloadPackage(ctx context.Context, repo, arch, name string, writer io.Writer) error {
	resp, err := c.request.Transport().Get(
		ctx,
		c.request.Transport().Endpoint("repo", repo, arch, name),
		false,
	)
	if err != nil {
		return errors.WrapErr(err, "download "+name)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return httpclient.ReadResponseError(resp, "download "+name)
	}
	if _, err := io.Copy(writer, resp.Body); err != nil {
		return errors.WrapErr(err, "write "+name)
	}
	return nil
}

func (c *Client) DownloadPackageFile(ctx context.Context, repo, arch, name, destination string) error {
	return safefile.Replace(destination, 0o644, func(file io.Writer) error { //nolint:gosec // downloaded package artifacts are public
		return c.DownloadPackage(ctx, repo, arch, name, file)
	})
}
