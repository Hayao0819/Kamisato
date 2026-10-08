package publishcmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/Hayao0819/Kamisato/ayaka/cmd/internal/keyinput"
	"github.com/Hayao0819/Kamisato/ayaka/cmd/internal/remote"
	"github.com/Hayao0819/Kamisato/ayaka/cmd/keyring/internal/buildflags"
	"github.com/Hayao0819/Kamisato/ayaka/keyring"
	"github.com/Hayao0819/Kamisato/internal/errors"
	"github.com/spf13/cobra"
)

func Cmd() *cobra.Command {
	var params keyring.BuildParams
	cmd := &cobra.Command{
		Use:   "publish <repo>",
		Short: "Build the keyring package and upload it to a repository on ayato",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			repo := args[0]
			k, _, err := keyinput.DefaultProvider().Load(keyinput.Read(cmd))
			if err != nil {
				return err
			}
			client, err := remote.DefaultProvider().RepoClient(cmd)
			if err != nil {
				return err
			}

			tmp, err := os.MkdirTemp("", "ayaka-keyring-")
			if err != nil {
				return errors.WrapErr(err, "create temp dir")
			}
			defer func() { _ = os.RemoveAll(tmp) }()

			pkgPath, sigPath, err := keyring.MakePackage(cmd.Context(), k, params, tmp)
			if err != nil {
				return err
			}
			if err := client.UploadPackageFiles(cmd.Context(), repo, pkgPath, sigPath); err != nil {
				return errors.WrapErr(err, "failed to upload keyring package")
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Published %s to %s\n", filepath.Base(pkgPath), repo)
			return nil
		},
	}
	buildflags.Add(cmd, &params)
	remote.AddRepoServerFlags(cmd)
	return cmd
}
