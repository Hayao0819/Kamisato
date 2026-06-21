package domain

import (
	"time"

	"github.com/Hayao0819/Kamisato/miko/joblog"
)

// JobStatus represents the lifecycle state of a build job.
type JobStatus string

const (
	JobStatusQueued    JobStatus = "queued"
	JobStatusRunning   JobStatus = "running"
	JobStatusSuccess   JobStatus = "success"
	JobStatusFailed    JobStatus = "failed"
	JobStatusCancelled JobStatus = "cancelled"
)

// BuildRequest is the input for submitting a build.
type BuildRequest struct {
	Repo string `json:"repo"`
	Arch string `json:"arch"`
	// Git clones a git/AUR repository as the build source. Mutually exclusive
	// with Pkgbuild; exactly one source must be provided.
	Git *GitSource `json:"git,omitempty"`
	// Pkgbuild is the raw PKGBUILD contents, used as the build source when Git
	// is not set.
	Pkgbuild string `json:"pkgbuild,omitempty"`
	// Files are extra filename->contents written alongside the Pkgbuild source.
	Files map[string]string `json:"files,omitempty"`
	// InstallPkgs are local package files installed before building.
	InstallPkgs []string `json:"install_pkgs"`
	// GPGKey identifies the signing key to use after build.
	GPGKey string `json:"gpg_key"`
	// Timeout in minutes; 0 uses the server default.
	Timeout int `json:"timeout,omitempty"`
}

// GitSource describes a git/AUR repository to clone as the build source.
type GitSource struct {
	URL    string `json:"url"`
	Ref    string `json:"ref,omitempty"`
	Subdir string `json:"subdir,omitempty"`
}

// BuildJob tracks a single build through the queue and worker.
type BuildJob struct {
	ID       string    `json:"id"`
	Repo     string    `json:"repo"`
	Arch     string    `json:"arch"`
	Status   JobStatus `json:"status"`
	Logs     string    `json:"logs"`
	Err      string    `json:"err,omitempty"`
	Packages []string  `json:"packages,omitempty"`
	Retries  int       `json:"retries,omitempty"`

	Request   *BuildRequest `json:"-"`
	CreatedAt time.Time     `json:"created_at"`
	StartedAt *time.Time    `json:"started_at,omitempty"`
	EndedAt   *time.Time    `json:"ended_at,omitempty"`

	// Log is the live build-log buffer, populated by the worker and streamed by
	// the logs endpoint. Set by the service on Submit. Not serialized.
	Log *joblog.Buffer `json:"-"`
}

// BuildStats is a snapshot of the build service served by the stats endpoint.
type BuildStats struct {
	Workers     int               `json:"workers"`
	QueueLength int               `json:"queue_length"`
	Running     int               `json:"running"`
	Counts      map[JobStatus]int `json:"counts"`
	Total       int               `json:"total"`
	SuccessRate float64           `json:"success_rate"` // success/(success+failed); 0 when none finished
	UptimeSec   int               `json:"uptime_sec"`
}
