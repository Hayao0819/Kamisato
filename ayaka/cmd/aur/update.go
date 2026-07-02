package aurcmd

import "github.com/spf13/cobra"

// aurUpdateCmd pulls tracked AUR packages from upstream, cloning any that are
// not present yet.
func aurUpdateCmd() *cobra.Command {
	return aurFetchCmd(
		"update <repo> <aur-pkg> [aur-pkg...]",
		"Update tracked AUR packages from upstream",
	)
}
