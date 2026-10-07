package nvcheckcmd

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/Hayao0819/Kamisato/ayaka/app"
	"github.com/Hayao0819/Kamisato/ayaka/service/source"
	cmdline "github.com/Hayao0819/Kamisato/internal/cli"
	pacmansource "github.com/Hayao0819/Kamisato/internal/pacman/source"
	"github.com/Hayao0819/Kamisato/internal/pacman/source/nvcheck"
)

// nvChecker is the slice of service/source this command drives.
type nvChecker interface {
	RunNvCheck(ctx context.Context, srcrepo *pacmansource.SourceRepo, client *http.Client) []source.CheckResult
}

type sourceNvChecker struct{}

func (sourceNvChecker) RunNvCheck(ctx context.Context, srcrepo *pacmansource.SourceRepo, client *http.Client) []source.CheckResult {
	return source.RunNvCheck(ctx, srcrepo, client)
}

type row struct {
	Repo    string `json:"repo"`
	Pkgbase string `json:"pkgbase"`
	Current string `json:"current"`
	Latest  string `json:"latest"`
	Method  string `json:"method"`
	Status  string `json:"status"`
}

const defaultFmt = "table {{.Repo}}\t{{.Pkgbase}}\t{{.Current}}\t{{.Latest}}\t{{.Method}}\t{{.Status}}"

// Cmd checks every package holding a .nvchecker.toml against its upstream, so
// a scheduled CI run can bump and rebuild what moved. Exits non-zero when any
// package is out of date.
func Cmd(runtime *app.Runtime) *cobra.Command { return newCommand(sourceNvChecker{}, runtime) }

func newCommand(svc nvChecker, runtime *app.Runtime) *cobra.Command {
	cmd := cobra.Command{
		Use:   "nvcheck",
		Short: "Check packages with a .nvchecker.toml for newer upstream versions",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			a, err := runtime.App()
			if err != nil {
				return err
			}
			client := &http.Client{Timeout: 30 * time.Second}
			if tok := os.Getenv("GITHUB_TOKEN"); tok != "" {
				client = nvcheck.WithGitHubToken(client, tok)
			}
			format, err := cmdline.ResolveFormat(cmd, defaultFmt)
			if err != nil {
				return err
			}

			var rows []row
			outdated := 0
			for _, srcrepo := range a.SrcRepos {
				for _, r := range svc.RunNvCheck(cmd.Context(), srcrepo, client) {
					status := "up-to-date"
					switch {
					case r.Err != nil:
						status = "error: " + r.Err.Error()
					case r.Outdated:
						status = "OUTDATED"
						outdated++
					}
					rows = append(rows, row{
						Repo:    srcrepo.Config.Name,
						Pkgbase: r.Pkgbase,
						Current: dashIfEmpty(r.Current),
						Latest:  dashIfEmpty(r.Latest),
						Method:  string(r.Method),
						Status:  status,
					})
				}
			}

			header := row{Repo: "REPO", Pkgbase: "PKGBASE", Current: "CURRENT", Latest: "LATEST", Method: "METHOD", Status: "STATUS"}
			if err := cmdline.RenderList(cmd.OutOrStdout(), format, header, rows); err != nil {
				return err
			}
			if outdated > 0 {
				return fmt.Errorf("%d package(s) out of date", outdated)
			}
			return nil
		},
	}
	cmdline.AddFormatFlags(&cmd)
	return &cmd
}

func dashIfEmpty(s string) string {
	if s == "" {
		return "-"
	}
	return s
}
