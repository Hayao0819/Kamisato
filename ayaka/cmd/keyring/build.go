package keyringcmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/Hayao0819/Kamisato/ayaka/cmd/shared"
)

func buildCmd() *cobra.Command {
	var (
		params buildParams
		outDir string
	)
	cmd := &cobra.Command{
		Use:   "build",
		Short: "Build the keyring package into a local directory",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			k, _, err := shared.LoadSigningKey(cmd)
			if err != nil {
				return err
			}
			pkgPath, sigPath, err := makePackage(k, params, outDir)
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
	addBuildFlags(cmd, &params)
	cmd.Flags().StringVarP(&outDir, "output", "o", ".", "Directory to write the package into")
	return cmd
}
