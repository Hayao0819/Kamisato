package shared

import (
	"context"
	"strings"

	"github.com/Hayao0819/Kamisato/internal/client"
	"github.com/Hayao0819/Kamisato/pkg/pacman/alpm"
	"github.com/Hayao0819/Kamisato/pkg/pacman/repo"
)

// PkgRow is one `ayaka list` row; its fields are the columns a --format template references (e.g. {{.Package}}).
type PkgRow struct {
	Repo      string `json:"repo"`
	Package   string `json:"package"`
	Installed string `json:"installed"`
	Local     string `json:"local"`
	Remote    string `json:"remote"`
	Build     string `json:"build"`
}

// DefaultListFormat is the Docker-style `table` form: every column, aligned, with a header.
const DefaultListFormat = "table {{.Package}}\t{{.Installed}}\t{{.Local}}\t{{.Remote}}\t{{.Build}}"

// BuildPkgRows builds one row per source package. Remote version, build status,
// and installed version are fetched only when the format references them, so a
// local-only format stays fast and offline.
func BuildPkgRows(repos []*repo.SourceRepo, format, server string) []PkgRow {
	wantRemote := formatNeeds(format, "Remote")
	wantBuild := formatNeeds(format, "Build")
	wantInstalled := formatNeeds(format, "Installed")

	var installed map[string]string
	if wantInstalled {
		installed, _ = alpm.InstalledVersions()
	}
	var jobs []client.Job
	if wantBuild {
		if api := ayatoClientBestEffort(server); api != nil {
			jobs, _ = api.ListJobs(context.Background())
		}
	}

	var rows []PkgRow
	for _, r := range repos {
		var remote *repo.RemoteRepo
		if wantRemote && r.Config.URL != "" {
			// Config.URL is arch-less; the list column reports the default x86_64
			// remote, matching the build command's default --arch.
			remote, _ = repo.RepoFromURL(strings.TrimRight(r.Config.URL, "/")+"/x86_64", r.Config.Name)
		}

		for _, p := range r.Pkgs {
			row := PkgRow{
				Repo:    r.Config.Name,
				Package: p.Base(),
				Local:   orDash(p.Version()),
			}
			if wantInstalled {
				row.Installed = orDash(firstInstalled(installed, p.Names()))
			}
			if wantRemote {
				ver := ""
				if remote != nil {
					if bp := remote.PkgByPkgBase(p.Base()); bp != nil {
						ver = bp.Version()
					}
				}
				row.Remote = orDash(ver)
			}
			if wantBuild {
				row.Build = orDash(LatestJobStatus(jobs, r.Config.Name, p.Names()))
			}
			rows = append(rows, row)
		}
	}
	return rows
}

func formatNeeds(format, field string) bool {
	if format == "json" {
		return true
	}
	return strings.Contains(format, field)
}

func firstInstalled(installed map[string]string, names []string) string {
	for _, n := range names {
		if v, ok := installed[n]; ok {
			return v
		}
	}
	return ""
}

// LatestJobStatus returns the status of the latest miko job for the package. A
// job matches on repo and either a named package or a whole-repo build (no
// packages listed).
func LatestJobStatus(jobs []client.Job, repoName string, names []string) string {
	want := make(map[string]bool, len(names))
	for _, n := range names {
		want[n] = true
	}

	status, latest := "", ""
	for _, j := range jobs {
		if j.Repo != repoName {
			continue
		}
		match := len(j.Packages) == 0
		for _, pn := range j.Packages {
			if want[pn] {
				match = true
				break
			}
		}
		if !match {
			continue
		}
		// CreatedAt is RFC3339, which sorts lexically by time.
		if j.CreatedAt >= latest {
			latest = j.CreatedAt
			status = string(j.Status)
		}
	}
	return status
}

// ayatoBaseBestEffort resolves the ayato base URL and CLI token for the build
// column: the registered --server (or serverdb default). Returns empty strings
// when no registered server is available, so the caller skips the job lookup —
// which the auth-gated jobs endpoint would reject without the token anyway.
func ayatoClientBestEffort(server string) *client.Ayato {
	srv, err := ResolveAyatoServer(server)
	if err != nil {
		return nil
	}
	api, err := AyatoClient(srv)
	if err != nil {
		return nil
	}
	return api
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}
