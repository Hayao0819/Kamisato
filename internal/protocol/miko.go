// Package protocol contains shared HTTP wire types.
package protocol

type JobStatus string

const (
	JobStatusQueued    JobStatus = "queued"
	JobStatusRunning   JobStatus = "running"
	JobStatusSuccess   JobStatus = "success"
	JobStatusFailed    JobStatus = "failed"
	JobStatusCancelled JobStatus = "cancelled"
)

type BuildReason string

const (
	ReasonManual        BuildReason = "manual"
	ReasonDependency    BuildReason = "dependency"
	ReasonVersionUpdate BuildReason = "version_update"
	ReasonSonameRebuild BuildReason = "soname_rebuild"
	ReasonRetry         BuildReason = "retry"
)

type BuildRequest struct {
	Repo        string            `json:"repo"`
	Arch        string            `json:"arch"`
	Microarch   string            `json:"microarch,omitempty"`
	Git         *GitSource        `json:"git,omitempty"`
	Pkgbuild    string            `json:"pkgbuild,omitempty"`
	Files       map[string]string `json:"files,omitempty"`
	InstallPkgs []string          `json:"install_pkgs"`
	SignMode    string            `json:"sign_mode,omitempty"`
	Timeout     int               `json:"timeout,omitempty"`
}

const (
	SignHost   = "host"
	SignClient = "client"
)

type GitSource struct {
	URL    string `json:"url"`
	Ref    string `json:"ref,omitempty"`
	Subdir string `json:"subdir,omitempty"`
}

// BuildJob is the public job projection.
type BuildJob struct {
	ID        string      `json:"id"`
	Repo      string      `json:"repo"`
	Arch      string      `json:"arch"`
	Status    JobStatus   `json:"status"`
	Logs      string      `json:"logs"`
	Err       string      `json:"err,omitempty"`
	Packages  []string    `json:"packages,omitempty"`
	Retries   int         `json:"retries,omitempty"`
	Reason    BuildReason `json:"reason,omitempty"`
	CreatedAt string      `json:"created_at"`
	StartedAt *string     `json:"started_at,omitempty"`
	EndedAt   *string     `json:"ended_at,omitempty"`
}

type BuildStats struct {
	Workers     int               `json:"workers"`
	QueueLength int               `json:"queue_length"`
	Running     int               `json:"running"`
	Counts      map[JobStatus]int `json:"counts"`
	Total       int               `json:"total"`
	SuccessRate float64           `json:"success_rate"`
	UptimeSec   int64             `json:"uptime_sec"`
}
