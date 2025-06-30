package cmd

import (
	"os"
	"os/exec"
	"path"

	"github.com/Hayao0819/Kamisato/pkg/pacman/repo"
	"github.com/spf13/cobra"
)

func updateSrcinfoCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "update-srcinfo",
		Aliases: []string{"us"},
		Short:   "Update srcinfo files",
		Long:    "Update srcinfo files",
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
				cmd.Println("Updated srcinfo file in", dir)
			}
			return nil
		},
	}

	return cmd
}

func init() {
	subCmds = append(subCmds, updateSrcinfoCmd())
}
