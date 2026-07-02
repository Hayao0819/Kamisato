package domain

import (
	"time"
)

type JobStatus string

const (
	JobStatusQueued    JobStatus = "queued"
	JobStatusRunning   JobStatus = "running"
	JobStatusSuccess   JobStatus = "success"
	JobStatusFailed    JobStatus = "failed"
	JobStatusCancelled JobStatus = "cancelled"
)

// BuildReason records why a build was triggered so a job's origin is visible in
// its status and logs.
type BuildReason string

const (
	// ReasonManual is a build a user explicitly submitted.
	ReasonManual BuildReason = "manual"
	// ReasonDependency is an AUR dependency built to satisfy another build.
	ReasonDependency BuildReason = "dependency"
	// ReasonVersionUpdate is a rebuild triggered by a newer upstream version.
	ReasonVersionUpdate BuildReason = "version_update"
	// ReasonSonameRebuild is a rebuild triggered by a soname bump in a dependency.
	ReasonSonameRebuild BuildReason = "soname_rebuild"
	// ReasonRetry is a re-attempt of a build that failed transiently.
	ReasonRetry BuildReason = "retry"
)

type BuildRequest struct {
	Repo string `json:"repo"`
	Arch string `json:"arch"`
	// Microarch, when set, targets an x86-64 feature level (x86_64_v2/v3/v4). It
	// requires Arch x86_64; empty builds at the arch's baseline.
	Microarch string `json:"microarch,omitempty"`
	// Mutually exclusive with Pkgbuild; exactly one source must be provided.
	Git *GitSource `json:"git,omitempty"`
	// Pkgbuild is the raw PKGBUILD contents.
	Pkgbuild string `json:"pkgbuild,omitempty"`
	// Files are extra filename->contents written alongside the Pkgbuild source.
	Files map[string]string `json:"files,omitempty"`
	// InstallPkgs are local package files installed before building.
	InstallPkgs []string `json:"install_pkgs"`
	// SignMode selects where signing happens: "host" (default) signs on the
	// worker with its host key; "client" leaves the artifact unsigned for the
	// requester to download and sign locally.
	SignMode string `json:"sign_mode,omitempty"`
	// Timeout in minutes; 0 uses the server default.
	Timeout int `json:"timeout,omitempty"`
}

const (
	SignHost   = "host"
	SignClient = "client"
)

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
	// Reason records why this build was triggered (manual submit, dependency,
	// retry, ...).
	Reason BuildReason `json:"reason,omitempty"`

	Request *BuildRequest `json:"-"`
	// ArtifactDir is the retained build-output dir for a client-signed job, served
	// for download; empty otherwise. Server-internal, never serialized.
	ArtifactDir string     `json:"-"`
	CreatedAt   time.Time  `json:"created_at"`
	StartedAt   *time.Time `json:"started_at,omitempty"`
	EndedAt     *time.Time `json:"ended_at,omitempty"`
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
