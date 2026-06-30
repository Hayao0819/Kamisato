package ayatoclient

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Hayao0819/Kamisato/internal/utils"
)

// SubmitBuild posts a build request to ayato with a Bearer CLI token and returns
// the job id assigned by miko.
func SubmitBuild(base, token string, req *BuildRequest) (string, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return "", utils.WrapErr(err, "failed to encode build request")
	}

	httpReq, err := http.NewRequest(http.MethodPost, endpoint(base, "/api/unstable/build"), bytes.NewReader(body))
	if err != nil {
		return "", utils.WrapErr(err, "failed to create build request")
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return "", utils.WrapErr(err, "failed to submit build")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		return "", responseErr(resp, "build submit")
	}

	var out struct {
		JobID string `json:"job_id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", utils.WrapErr(err, "failed to decode build response")
	}
	return out.JobID, nil
}

// ListJobs fetches all jobs from ayato. The jobs endpoint is public.
func ListJobs(base string) ([]Job, error) {
	resp, err := http.Get(endpoint(base, "/api/unstable/jobs"))
	if err != nil {
		return nil, utils.WrapErr(err, "failed to list jobs")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, responseErr(resp, "list jobs")
	}

	var jobs []Job
	if err := json.NewDecoder(resp.Body).Decode(&jobs); err != nil {
		return nil, utils.WrapErr(err, "failed to decode jobs")
	}
	return jobs, nil
}

// JobStatus fetches a single job by id from ayato. The endpoint is public.
func JobStatus(base, id string) (*Job, error) {
	resp, err := http.Get(endpoint(base, "/api/unstable/jobs/"+id))
	if err != nil {
		return nil, utils.WrapErr(err, "failed to get job status")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, responseErr(resp, "job status")
	}

	var job Job
	if err := json.NewDecoder(resp.Body).Decode(&job); err != nil {
		return nil, utils.WrapErr(err, "failed to decode job")
	}
	return &job, nil
}

// WaitJob blocks until the job reaches a terminal state, streaming its build
// logs to logs (best-effort) once the job starts running, and returns the final
// job. A nil logs writer disables log streaming. It returns ctx.Err() when ctx
// is cancelled, so a job stuck in queued (or an unknown status) cannot hang the
// caller forever.
func WaitJob(ctx context.Context, base, id string, logs io.Writer) (*Job, error) {
	streamed := false
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		job, err := JobStatus(base, id)
		if err != nil {
			return nil, err
		}
		switch job.Status {
		case "success", "failed", "cancelled":
			if logs != nil && !streamed {
				_ = StreamLogs(base, id, logs) // a fast job: dump whatever buffered
			}
			return job, nil
		case "running":
			if logs != nil && !streamed {
				streamed = true
				_ = StreamLogs(base, id, logs) // blocks until the stream closes
				continue                       // re-fetch the now-terminal status
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
func CancelJob(base, token, id string) error {
	httpReq, err := http.NewRequest(http.MethodDelete, endpoint(base, "/api/unstable/jobs/"+id), nil)
	if err != nil {
		return utils.WrapErr(err, "failed to create cancel request")
	}
	httpReq.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return utils.WrapErr(err, "failed to cancel job")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return responseErr(resp, "cancel job")
	}
	return nil
}

// FetchStats fetches miko's build statistics from ayato (public endpoint).
func FetchStats(base string) (*Stats, error) {
	resp, err := http.Get(endpoint(base, "/api/unstable/stats"))
	if err != nil {
		return nil, utils.WrapErr(err, "failed to get stats")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, responseErr(resp, "stats")
	}

	var stats Stats
	if err := json.NewDecoder(resp.Body).Decode(&stats); err != nil {
		return nil, utils.WrapErr(err, "failed to decode stats")
	}
	return &stats, nil
}

// StreamLogs reads the Server-Sent Events log stream for a job and writes each
// log line to w. The logs endpoint is public, so no auth is sent. It returns
// when the stream is closed by the server.
func StreamLogs(base, id string, w io.Writer) error {
	resp, err := http.Get(endpoint(base, "/api/unstable/jobs/"+id+"/logs"))
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
