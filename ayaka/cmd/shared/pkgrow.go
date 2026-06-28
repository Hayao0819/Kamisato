package shared

import (
	"strings"

	blinky_util "github.com/BrenekH/blinky/cmd/blinky/util"
	"github.com/Hayao0819/Kamisato/internal/ayatoclient"
	"github.com/Hayao0819/Kamisato/pkg/pacman/alpm"
	"github.com/Hayao0819/Kamisato/pkg/pacman/repo"
)

// PkgRow is one line of `ayaka list` output. Its fields are the columns a
// --format template can reference (e.g. {{.Package}}).
type PkgRow struct {
	Repo      string `json:"repo"`
	Package   string `json:"package"`
	Installed string `json:"installed"`
	Local     string `json:"local"`
	Remote    string `json:"remote"`
	Build     string `json:"build"`
}

// DefaultListFormat shows every column with a header. It is the Docker-style
// `table` form so the output aligns and is labelled by default.
const DefaultListFormat = "table {{.Package}}\t{{.Installed}}\t{{.Local}}\t{{.Remote}}\t{{.Build}}"

// BuildPkgRows assembles one row per source package. The remote version, miko
// build status, and installed version are gathered best-effort and only when
// the format actually references those columns, so a local-only format stays
// fast and works offline.
func BuildPkgRows(repos []*repo.SourceRepo, format, server string) []PkgRow {
	wantRemote := formatNeeds(format, "Remote")
	wantBuild := formatNeeds(format, "Build")
	wantInstalled := formatNeeds(format, "Installed")

	var installed map[string]string
	if wantInstalled {
		installed, _ = alpm.InstalledVersions()
	}
	var jobs []ayatoclient.Job
	if wantBuild {
		if base := ayatoBaseBestEffort(server); base != "" {
			jobs, _ = ayatoclient.ListJobs(base)
		}
	}

	var rows []PkgRow
	for _, r := range repos {
		var remote *repo.RemoteRepo
		if wantRemote && r.Config.Server != "" {
			remote, _ = repo.RepoFromURL(r.Config.Server, r.Config.Name)
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

// formatNeeds reports whether the format string references the given field,
// so the expensive lookup behind that column can be skipped otherwise.
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

// LatestJobStatus returns the status of the most recent miko job that built the
// package in the repo. A job matches when its repo agrees and it either targets
// one of the package names or is a whole-repo build (no packages listed).
func LatestJobStatus(jobs []ayatoclient.Job, repoName string, names []string) string {
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
			status = j.Status
		}
	}
	return status
}

// ayatoBaseBestEffort returns the ayato base URL for build-status lookups: the
// --server value, else the serverdb default, else "" when neither is set.
func ayatoBaseBestEffort(server string) string {
	if server != "" {
		return server
	}
	db, err := blinky_util.ReadServerDB()
	if err != nil {
		return ""
	}
	return db.DefaultServer
}

// orDash renders an empty value as "-" so columns stay aligned and obviously
// empty.
func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}
