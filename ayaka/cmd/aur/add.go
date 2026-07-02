package aurcmd

import "github.com/spf13/cobra"

// aurAddCmd clones AUR packages into a source repository, falling back to a pull
// when one is already present so it is safe to re-run.
func aurAddCmd() *cobra.Command {
	return aurFetchCmd(
		"add <repo> <aur-pkg> [aur-pkg...]",
		"Add AUR packages to a source repository",
	)
}
