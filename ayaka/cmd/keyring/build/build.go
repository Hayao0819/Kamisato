package buildcmd

import (
	"fmt"

	"github.com/Hayao0819/Kamisato/ayaka/cmd/internal/keyinput"
	"github.com/Hayao0819/Kamisato/ayaka/cmd/keyring/internal/buildflags"
	"github.com/Hayao0819/Kamisato/ayaka/keyring"
	"github.com/spf13/cobra"
)

func Cmd() *cobra.Command {
	var (
		params keyring.BuildParams
		outDir string
	)
	cmd := &cobra.Command{
		Use:   "build",
		Short: "Build the keyring package into a local directory",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			k, _, err := keyinput.DefaultProvider().Load(keyinput.Read(cmd))
			if err != nil {
				return err
			}
			pkgPath, sigPath, err := keyring.MakePackage(cmd.Context(), k, params, outDir)
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			fmt.Fprintln(out, pkgPath)
			if sigPath != "" {
				fmt.Fprintln(out, sigPath)
			}
			return nil
		},
	}
	buildflags.Add(cmd, &params)
	cmd.Flags().StringVar(&outDir, "output-dir", ".", "Directory to write the package into")
	return cmd
}
