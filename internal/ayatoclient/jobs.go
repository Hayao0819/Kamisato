package ayatoclient

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Hayao0819/Kamisato/internal/utils"
)

// SubmitBuild posts a build request to ayato with a Bearer CLI token and returns
// the job id assigned by miko.
func SubmitBuild(ctx context.Context, base, token string, req *BuildRequest) (string, error) {
	var out struct {
		JobID string `json:"job_id"`
	}
	if err := doJSON(ctx, http.MethodPost, endpoint(base, "/api/unstable/build"), token, req, &out, http.StatusAccepted, "build submit"); err != nil {
		return "", err
	}
	return out.JobID, nil
}

// ListJobs fetches all jobs. The read endpoints are auth-gated (admin CLI token
// via ayato, or the miko API key in direct mode), so token is always sent.
func ListJobs(ctx context.Context, base, token string) ([]Job, error) {
	var jobs []Job
	if err := doJSON(ctx, http.MethodGet, endpoint(base, "/api/unstable/jobs"), token, nil, &jobs, http.StatusOK, "list jobs"); err != nil {
		return nil, err
	}
	return jobs, nil
}

// JobStatus fetches a single job by id.
func JobStatus(ctx context.Context, base, token, id string) (*Job, error) {
	var job Job
	if err := doJSON(ctx, http.MethodGet, endpoint(base, "/api/unstable/jobs/"+id), token, nil, &job, http.StatusOK, "job status"); err != nil {
		return nil, err
	}
	return &job, nil
}

// WaitJob blocks until the job reaches a terminal state, streaming its build
// logs to logs (best-effort) once the job starts running, and returns the final
// job. A nil logs writer disables log streaming. It returns ctx.Err() when ctx
// is cancelled, so a job stuck in queued (or an unknown status) cannot hang the
// caller forever.
func WaitJob(ctx context.Context, base, token, id string, logs io.Writer) (*Job, error) {
	streamed := false
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		job, err := JobStatus(ctx, base, token, id)
		if err != nil {
			return nil, err
		}
		switch job.Status {
		case "success", "failed", "cancelled":
			if logs != nil && !streamed {
				_ = StreamLogs(ctx, base, token, id, logs) // a fast job: dump whatever buffered
			}
			return job, nil
		case "running":
			if logs != nil && !streamed {
				streamed = true
				_ = StreamLogs(ctx, base, token, id, logs) // blocks until the stream closes
				continue                                   // re-fetch the now-terminal status
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

// CancelJob requests cancellation of a job through ayato's authenticated proxy.
func CancelJob(ctx context.Context, base, token, id string) error {
	return doJSON(ctx, http.MethodDelete, endpoint(base, "/api/unstable/jobs/"+id), token, nil, nil, http.StatusOK, "cancel job")
}

// FetchStats fetches miko's build statistics (auth-gated).
func FetchStats(ctx context.Context, base, token string) (*Stats, error) {
	var stats Stats
	if err := doJSON(ctx, http.MethodGet, endpoint(base, "/api/unstable/stats"), token, nil, &stats, http.StatusOK, "stats"); err != nil {
		return nil, err
	}
	return &stats, nil
}

// StreamLogs reads the Server-Sent Events log stream for a job and writes each
// log line to w. The logs endpoint is public on ayato but auth-gated on a direct
// miko, so token is sent when set. It returns when the server closes the stream.
func StreamLogs(ctx context.Context, base, token, id string, w io.Writer) error {
	resp, err := get(ctx, streamClient, endpoint(base, "/api/unstable/jobs/"+id+"/logs"), token)
	if err != nil {
		return utils.WrapErr(err, "failed to open log stream")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return responseErr(resp, "job logs")
	}

	// miko falls back to plain text when a job has no live buffer; in that case
	// the content type is not event-stream and we copy the body verbatim.
	if !strings.HasPrefix(resp.Header.Get("Content-Type"), "text/event-stream") {
		if _, err := io.Copy(w, resp.Body); err != nil {
			return utils.WrapErr(err, "failed to read logs")
		}
		return nil
	}

	scanner := bufio.NewScanner(resp.Body)
	// Build logs can produce long lines; allow up to 1MiB per line.
	scanner.Buffer(make([]byte, 0, 64*1024), 1<<20)
	for scanner.Scan() {
		line := scanner.Text()
		// SSE frames carry the payload on "data:" lines; blank lines separate
		// frames and other field lines are not used here.
		data, ok := strings.CutPrefix(line, "data:")
		if !ok {
			continue
		}
		data = strings.TrimPrefix(data, " ")
		if _, err := fmt.Fprintln(w, data); err != nil {
			return utils.WrapErr(err, "failed to write log line")
		}
	}
	if err := scanner.Err(); err != nil {
		return utils.WrapErr(err, "failed to read log stream")
	}
	return nil
}
