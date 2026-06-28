package domain

import (
	"time"

	"github.com/Hayao0819/Kamisato/miko/joblog"
)

type JobStatus string

const (
	JobStatusQueued    JobStatus = "queued"
	JobStatusRunning   JobStatus = "running"
	JobStatusSuccess   JobStatus = "success"
	JobStatusFailed    JobStatus = "failed"
	JobStatusCancelled JobStatus = "cancelled"
)

type BuildRequest struct {
	Repo string `json:"repo"`
	Arch string `json:"arch"`
	// Mutually exclusive with Pkgbuild; exactly one source must be provided.
	Git *GitSource `json:"git,omitempty"`
	// Pkgbuild is the raw PKGBUILD contents.
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

	// Log is the live build-log buffer, set by the service on Submit.
	Log *joblog.Buffer `json:"-"`
}

type BuildStats struct {
	Workers     int               `json:"workers"`
	QueueLength int               `json:"queue_length"`
	Running     int               `json:"running"`
	Counts      map[JobStatus]int `json:"counts"`
	Total       int               `json:"total"`
	SuccessRate float64           `json:"success_rate"` // success/(success+failed); 0 when none finished
	UptimeSec   int               `json:"uptime_sec"`
}
