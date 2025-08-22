package cmd

import (
	"os"
	"os/exec"
	"path"

	"github.com/Hayao0819/Kamisato/pkg/pacman/repo"
	"github.com/spf13/cobra"
)

// updateSrcinfoCmd returns the command to update .SRCINFO files in all source directories.
// Returns the command to update .SRCINFO files in all source directories.
func updateSrcinfoCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "update-srcinfo",
		Aliases: []string{"us"},
		Short:   "Update all SRCINFO files",
		Long:    "Update .SRCINFO files in all source directories.",
		RunE: func(cmd *cobra.Command, args []string) error {
			srcdirs, err := repo.GetSrcDirs(config.RepoDir)
			if err != nil {
				return err
			}
			for _, dir := range srcdirs {
				gencmd := exec.Command("makepkg", "--printsrcinfo")
				gencmd.Dir = dir

				srcinfoPath := path.Join(dir, ".SRCINFO")
				srcinfoFile, err := os.Create(srcinfoPath)
				if err != nil {
					return err
				}

				gencmd.Stdout = srcinfoFile
				gencmd.Stderr = cmd.ErrOrStderr()
				if err := gencmd.Run(); err != nil {
					return err
				}
				cmd.Println("Updated SRCINFO file:", dir)
			}
			return nil
		},
	}

	return cmd
}

func init() {
	subCmds = append(subCmds, updateSrcinfoCmd())
}
