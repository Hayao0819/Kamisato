package report

import (
	"fmt"
	"io"
	"strings"

	alpm "github.com/Hayao0819/dyalpm"
	"github.com/fatih/color"
)

type statusItem struct {
	name   string
	detail string
}

// PrintStatus writes the git-status-style build report grouped by attention
// category; withRepo prefixes package names with their repository.
func PrintStatus(out io.Writer, rows []Row, withRepo bool) {
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
	return alpm.VerCmp(local, remote) > 0
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
