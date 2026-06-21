package cmd

import (
	"encoding/json"
	"os/exec"
	"strings"
	"text/tabwriter"
	"text/template"

	blinky_util "github.com/BrenekH/blinky/cmd/blinky/util"
	"github.com/Hayao0819/Kamisato/internal/ayatoclient"
	"github.com/Hayao0819/Kamisato/internal/utils"
	"github.com/Hayao0819/Kamisato/pkg/pacman/repo"
	"github.com/spf13/cobra"
)

// pkgRow is one line of `ayaka list` output. Its fields are the columns a
// --format template can reference (e.g. {{.Package}}).
type pkgRow struct {
	Package   string `json:"package"`
	Installed string `json:"installed"`
	Local     string `json:"local"`
	Remote    string `json:"remote"`
	Build     string `json:"build"`
}

// pkgHeader is rendered through the format template to produce the table header
// row, the same way Docker derives headers from the format string.
var pkgHeader = pkgRow{
	Package:   "PACKAGE",
	Installed: "INSTALLED",
	Local:     "LOCAL",
	Remote:    "REMOTE",
	Build:     "BUILD",
}

// defaultListFormat shows every column with a header. It is the Docker-style
// `table` form so the output aligns and is labelled by default.
const defaultListFormat = "table {{.Package}}\t{{.Installed}}\t{{.Local}}\t{{.Remote}}\t{{.Build}}"

// listCmd lists the packages in the source repositories as a table, with their
// installed/local/remote versions and miko build status. The columns are
// selectable with a Docker-style --format template.
func listCmd() *cobra.Command {
	var format string
	var server string

	cmd := cobra.Command{
		Use:   "list [repo]",
		Short: "List source packages with their versions and build status",
		Long: "List source packages as a table.\n\n" +
			"Columns are chosen with a Go template via --format, like docker:\n" +
			"  ayaka list --format 'table {{.Package}}\\t{{.Local}}\\t{{.Remote}}'\n" +
			"  ayaka list --format '{{.Package}} {{.Build}}'\n" +
			"  ayaka list --format json\n\n" +
			"Fields: .Package .Installed .Local .Remote .Build",
		Args: cobra.MaximumNArgs(1),
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			if len(args) == 0 {
				return getSrcRepoNames(), cobra.ShellCompDirectiveNoFileComp
			}
			return nil, cobra.ShellCompDirectiveNoFileComp
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			repos := srcRepo
			if len(args) > 0 {
				argrepo := getSrcRepo(args[0])
				if argrepo == nil {
					return utils.WrapErr(ErrInvalidRepoName, args[0])
				}
				repos = []*repo.SourceRepo{argrepo}
			}

			if format == "" {
				format = defaultListFormat
			}
			rows := buildPkgRows(repos, format, server)
			return renderRows(cmd, format, rows)
		},
	}

	cmd.Flags().StringVar(&format, "format", "", "Format the output with a Go template (Docker-style; 'table ...' or 'json')")
	cmd.Flags().StringVarP(&server, "server", "s", "", "ayato server for build status (default: serverdb default)")
	return &cmd
}

// buildPkgRows assembles one row per source package. The remote version, miko
// build status, and installed version are gathered best-effort and only when
// the format actually references those columns, so a local-only format stays
// fast and works offline.
func buildPkgRows(repos []*repo.SourceRepo, format, server string) []pkgRow {
	wantRemote := formatNeeds(format, "Remote")
	wantBuild := formatNeeds(format, "Build")
	wantInstalled := formatNeeds(format, "Installed")

	var installed map[string]string
	if wantInstalled {
		installed = installedVersions()
	}
	var jobs []ayatoclient.Job
	if wantBuild {
		if base := ayatoBaseBestEffort(server); base != "" {
			jobs, _ = ayatoclient.ListJobs(base)
		}
	}

	var rows []pkgRow
	for _, r := range repos {
		var remote *repo.RemoteRepo
		if wantRemote && r.Config.Server != "" {
			remote, _ = repo.RepoFromURL(r.Config.Server, r.Config.Name)
		}

		for _, p := range r.Pkgs {
			row := pkgRow{
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
				row.Build = orDash(latestJobStatus(jobs, r.Config.Name, p.Names()))
			}
			rows = append(rows, row)
		}
	}
	return rows
}

// renderRows writes the rows according to the format string. "json" emits one
// JSON object per line; a "table " prefix aligns the columns under a header;
// any other template is executed per row with no header.
func renderRows(cmd *cobra.Command, format string, rows []pkgRow) error {
	out := cmd.OutOrStdout()

	if format == "json" {
		enc := json.NewEncoder(out)
		for _, row := range rows {
			if err := enc.Encode(row); err != nil {
				return utils.WrapErr(err, "failed to encode row")
			}
		}
		return nil
	}

	// Let users write \t and \n in the format string, as docker does.
	tmplText := strings.NewReplacer(`\t`, "\t", `\n`, "\n").Replace(format)

	isTable := strings.HasPrefix(tmplText, "table ")
	if isTable {
		tmplText = strings.TrimPrefix(tmplText, "table ")
	}

	tmpl, err := template.New("list").Funcs(template.FuncMap{
		"json": func(v any) (string, error) {
			b, err := json.Marshal(v)
			return string(b), err
		},
	}).Parse(tmplText)
	if err != nil {
		return utils.WrapErr(err, "invalid --format template")
	}

	if isTable {
		w := tabwriter.NewWriter(out, 0, 4, 2, ' ', 0)
		if err := tmpl.Execute(w, pkgHeader); err != nil {
			return utils.WrapErr(err, "failed to render header")
		}
		if _, err := w.Write([]byte("\n")); err != nil {
			return err
		}
		for _, row := range rows {
			if err := tmpl.Execute(w, row); err != nil {
				return utils.WrapErr(err, "failed to render row")
			}
			if _, err := w.Write([]byte("\n")); err != nil {
				return err
			}
		}
		return w.Flush()
	}

	for _, row := range rows {
		if err := tmpl.Execute(out, row); err != nil {
			return utils.WrapErr(err, "failed to render row")
		}
		if _, err := out.Write([]byte("\n")); err != nil {
			return err
		}
	}
	return nil
}

// formatNeeds reports whether the format string references the given field,
// so the expensive lookup behind that column can be skipped otherwise.
func formatNeeds(format, field string) bool {
	if format == "json" {
		return true
	}
	return strings.Contains(format, field)
}

// installedVersions maps installed package name to version via `pacman -Q`. It
// is best-effort: an error (no pacman, etc.) yields an empty map.
func installedVersions() map[string]string {
	out, err := exec.Command("pacman", "-Q").Output()
	if err != nil {
		return map[string]string{}
	}
	m := map[string]string{}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		f := strings.Fields(line)
		if len(f) >= 2 {
			m[f[0]] = f[1]
		}
	}
	return m
}

// firstInstalled returns the version of the first of names that is installed.
func firstInstalled(installed map[string]string, names []string) string {
	for _, n := range names {
		if v, ok := installed[n]; ok {
			return v
		}
	}
	return ""
}

// latestJobStatus returns the status of the most recent miko job that built the
// package in the repo. A job matches when its repo agrees and it either targets
// one of the package names or is a whole-repo build (no packages listed).
func latestJobStatus(jobs []ayatoclient.Job, repoName string, names []string) string {
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

func init() {
	subCmds.Add(listCmd())
}
