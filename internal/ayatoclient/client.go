// Package ayatoclient is a thin HTTP client for the ayato-exposed build/jobs
// API. Clients (lumine, ayaka) talk only to ayato, which proxies build and job
// requests to the internal miko build server; this package never contacts miko
// directly.
package ayatoclient

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/Hayao0819/Kamisato/internal/utils"
)

// BuildRequest mirrors the miko build request that ayato proxies unchanged.
type BuildRequest struct {
	Repo string `json:"repo"`
	Arch string `json:"arch"`
	// Git clones a git/AUR repository as the build source. Mutually exclusive
	// with Pkgbuild.
	Git *GitSource `json:"git,omitempty"`
	// Pkgbuild is the raw PKGBUILD contents, used when Git is not set.
	Pkgbuild string `json:"pkgbuild,omitempty"`
	// Files are extra filename->contents written alongside the Pkgbuild source.
	Files       map[string]string `json:"files,omitempty"`
	InstallPkgs []string          `json:"install_pkgs"`
	GPGKey      string            `json:"gpg_key"`
	Timeout     int               `json:"timeout,omitempty"` // minutes; 0 = miko default
}

// GitSource describes a git/AUR repository to clone as the build source.
type GitSource struct {
	URL    string `json:"url"`
	Ref    string `json:"ref,omitempty"`
	Subdir string `json:"subdir,omitempty"`
}

// Job is the subset of miko's job representation that the CLI displays. Unknown
// fields are ignored so the client tolerates miko adding more.
type Job struct {
	ID        string   `json:"id"`
	Repo      string   `json:"repo"`
	Arch      string   `json:"arch"`
	Status    string   `json:"status"`
	Err       string   `json:"err,omitempty"`
	Packages  []string `json:"packages,omitempty"`
	CreatedAt string   `json:"created_at"`
	Retries   int      `json:"retries,omitempty"`
}

// Stats mirrors miko's build statistics.
type Stats struct {
	Workers     int            `json:"workers"`
	QueueLength int            `json:"queue_length"`
	Running     int            `json:"running"`
	Counts      map[string]int `json:"counts"`
	Total       int            `json:"total"`
	SuccessRate float64        `json:"success_rate"`
	UptimeSec   int64          `json:"uptime_sec"`
}

// SubmitBuild posts a build request to ayato with HTTP Basic auth and returns
// the job id assigned by miko.
func SubmitBuild(base, user, pass string, req *BuildRequest) (string, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return "", utils.WrapErr(err, "failed to encode build request")
	}

	httpReq, err := http.NewRequest(http.MethodPost, endpoint(base, "/api/unstable/build"), bytes.NewReader(body))
	if err != nil {
		return "", utils.WrapErr(err, "failed to create build request")
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.SetBasicAuth(user, pass)

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

// CancelJob requests cancellation of a job through ayato's authenticated proxy.
func CancelJob(base, user, pass, id string) error {
	httpReq, err := http.NewRequest(http.MethodDelete, endpoint(base, "/api/unstable/jobs/"+id), nil)
	if err != nil {
		return utils.WrapErr(err, "failed to create cancel request")
	}
	httpReq.SetBasicAuth(user, pass)

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

// endpoint joins the ayato base URL with an API path, tolerating a trailing
// slash on the base.
func endpoint(base, p string) string {
	return strings.TrimRight(base, "/") + p
}

// responseErr builds an error from a non-success response, including any error
// message the server returned in its JSON body.
func responseErr(resp *http.Response, op string) error {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	var apiErr struct {
		Error   string `json:"error"`
		Message string `json:"message"`
	}
	msg := strings.TrimSpace(string(body))
	if json.Unmarshal(body, &apiErr) == nil {
		if apiErr.Error != "" {
			msg = apiErr.Error
		} else if apiErr.Message != "" {
			msg = apiErr.Message
		}
	}
	if msg == "" {
		return utils.NewErrf("%s failed: %s", op, resp.Status)
	}
	return utils.NewErrf("%s failed: %s: %s", op, resp.Status, msg)
}
