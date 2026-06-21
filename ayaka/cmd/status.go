package cmd

import (
	"fmt"
	"io"
	"strings"

	"github.com/Hayao0819/Kamisato/internal/utils"
	"github.com/Hayao0819/Kamisato/pkg/pacman/alpm"
	"github.com/Hayao0819/Kamisato/pkg/pacman/repo"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

type statusItem struct {
	name   string
	detail string
}

func statusCmd() *cobra.Command {
	var server string

	cmd := &cobra.Command{
		Use:   "status [repo]",
		Short: "Show packages that are out of date or failed to build",
		Long:  "Show, like git status, which source packages failed to build, are out of date (PKGBUILD ahead of the published package), are building, or were never published.",
		Args:  cobra.MaximumNArgs(1),
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

			// defaultListFormat references every column, so all are populated.
			rows := buildPkgRows(repos, defaultListFormat, server)
			printStatus(cmd.OutOrStdout(), rows, len(repos) > 1)
			return nil
		},
	}

	cmd.Flags().StringVarP(&server, "server", "s", "", "ayato server for build status (default: serverdb default)")
	return cmd
}

func printStatus(out io.Writer, rows []pkgRow, withRepo bool) {
	var failed, outdated, building, unpublished []statusItem
	clean := 0

	for _, row := range rows {
		name := row.Package
		if withRepo {
			name = row.Repo + "/" + row.Package
		}

		switch {
		case isFailedStatus(row.Build):
			failed = append(failed, statusItem{name, "build failed at " + row.Local})
		case isBuildingStatus(row.Build):
			building = append(building, statusItem{name, row.Build + " " + row.Local})
		case row.Remote == "-":
			unpublished = append(unpublished, statusItem{name, row.Local})
		case localAheadOfRemote(row.Local, row.Remote):
			outdated = append(outdated, statusItem{name, row.Remote + " -> " + row.Local})
		default:
			clean++
		}
	}

	width := itemWidth(failed, outdated, building, unpublished)

	printGroup(out, color.New(color.FgRed, color.Bold), "Build failed", "see 'ayaka miko jobs' for the failing job", failed, width)
	printGroup(out, color.New(color.FgYellow, color.Bold), "Out of date", "PKGBUILD is ahead of the published package; rebuild with 'ayaka build' or 'ayaka miko build'", outdated, width)
	printGroup(out, color.New(color.FgCyan, color.Bold), "Building", "currently building on miko", building, width)
	printGroup(out, color.New(color.FgBlue, color.Bold), "Not published", "never published to ayato", unpublished, width)

	attention := len(failed) + len(outdated) + len(building) + len(unpublished)
	if attention == 0 {
		fmt.Fprintln(out, color.GreenString("Everything up to date")+fmt.Sprintf(" (%d packages)", clean))
		return
	}
	fmt.Fprintf(out, "%d package(s) need attention, %d up to date\n", attention, clean)
}

func printGroup(out io.Writer, header *color.Color, label, hint string, items []statusItem, width int) {
	if len(items) == 0 {
		return
	}
	fmt.Fprintln(out, header.Sprint(label+":"))
	if hint != "" {
		fmt.Fprintln(out, "  "+color.New(color.Faint).Sprint("("+hint+")"))
	}
	for _, it := range items {
		pad := strings.Repeat(" ", width-len(it.name))
		fmt.Fprintf(out, "    %s%s  %s\n", it.name, pad, it.detail)
	}
	fmt.Fprintln(out)
}

func itemWidth(groups ...[]statusItem) int {
	w := 0
	for _, g := range groups {
		for _, it := range g {
			if len(it.name) > w {
				w = len(it.name)
			}
		}
	}
	return w
}

func localAheadOfRemote(local, remote string) bool {
	if local == "-" || remote == "-" {
		return false
	}
	cmp, err := alpm.VerCmp(local, remote)
	if err != nil {
		return false
	}
	return cmp > 0
}

func isFailedStatus(s string) bool {
	return s == "failed" || s == "error"
}

func isBuildingStatus(s string) bool {
	switch s {
	case "queued", "running", "building", "pending":
		return true
	}
	return false
}

func init() {
	subCmds.Add(statusCmd())
}
