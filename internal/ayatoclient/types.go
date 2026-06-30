package ayatoclient

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
	// SignMode "client" leaves the build unsigned for local download+signing;
	// empty/"host" signs on the worker.
	SignMode string `json:"sign_mode,omitempty"`
	Timeout  int    `json:"timeout,omitempty"` // minutes; 0 = miko default
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

// Admin is an allowlisted ayato admin (GitHub id + login).
type Admin struct {
	ID    int64  `json:"id"`
	Login string `json:"login"`
}
