package nvcheckcmd

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/Hayao0819/Kamisato/ayaka/cmd/internal/sourcerepos"
	"github.com/Hayao0819/Kamisato/ayaka/service/source"
	sourcerepo "github.com/Hayao0819/Kamisato/ayaka/source"
	cmdline "github.com/Hayao0819/Kamisato/internal/cli"
	"github.com/Hayao0819/Kamisato/internal/pacman/nvcheck"
	"github.com/spf13/cobra"
)

// checkFunc is the source-check operation this command drives.
type checkFunc func(ctx context.Context, srcrepo *sourcerepo.SourceRepo, client *http.Client) []source.CheckResult

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
func Cmd() *cobra.Command {
	return newCommand(source.RunNvCheck, nil)
}

func newCommand(check checkFunc, list sourcerepos.ListFunc) *cobra.Command {
	var cmd cobra.Command
	if list == nil {
		list = sourcerepos.ForCommand(&cmd).All
	}
	cmd = cobra.Command{
		Use:   "nvcheck",
		Short: "Check packages with a .nvchecker.toml for newer upstream versions",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			format, err := cmdline.ResolveFormat(cmd, defaultFmt)
			if err != nil {
				return err
			}
			return run(cmd.Context(), cmd.OutOrStdout(), list, check, format)
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

func run(ctx context.Context, out io.Writer, list sourcerepos.ListFunc, check checkFunc, format string) error {
	repos, err := list()
	if err != nil {
		return err
	}
	client := &http.Client{Timeout: 30 * time.Second}
	if tok := os.Getenv("GITHUB_TOKEN"); tok != "" {
		client = nvcheck.WithGitHubToken(client, tok)
	}

	var rows []row
	outdated := 0
	for _, srcrepo := range repos {
		for _, r := range check(ctx, srcrepo, client) {
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
	if err := cmdline.RenderList(out, format, header, rows); err != nil {
		return err
	}
	if outdated > 0 {
		return fmt.Errorf("%d package(s) out of date", outdated)
	}
	return nil
}
