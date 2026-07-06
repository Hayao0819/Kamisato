package keyringcmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/Hayao0819/Kamisato/ayaka/cmd/shared"
	"github.com/Hayao0819/Kamisato/internal/blinkyutils"
	"github.com/Hayao0819/Kamisato/internal/errwrap"
)

func publishCmd() *cobra.Command {
	var params buildParams
	cmd := &cobra.Command{
		Use:   "publish <repo>",
		Short: "Build the keyring package and upload it to a repository on ayato",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			repo := args[0]
			k, _, err := shared.LoadSigningKey(cmd)
			if err != nil {
				return err
			}
			client, err := shared.RepoClient(cmd)
			if err != nil {
				return err
			}

			tmp, err := os.MkdirTemp("", "ayaka-keyring-")
			if err != nil {
				return errwrap.WrapErr(err, "create temp dir")
			}
			defer func() { _ = os.RemoveAll(tmp) }()

			pkgPath, sigPath, err := makePackage(k, params, tmp)
			if err != nil {
				return err
			}
			if err := blinkyutils.Upload(client, repo, pkgPath, sigPath); err != nil {
				return errwrap.WrapErr(err, "failed to upload keyring package")
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Published %s to %s\n", filepath.Base(pkgPath), repo)
			return nil
		},
	}
	addBuildFlags(cmd, &params)
	shared.AddRepoServerFlags(cmd)
	return cmd
}
