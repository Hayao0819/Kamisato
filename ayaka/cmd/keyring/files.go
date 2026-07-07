package keyringcmd

import (
	"fmt"
	"path/filepath"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/spf13/cobra"

	"github.com/Hayao0819/Kamisato/ayaka/cmd/shared"
	"github.com/Hayao0819/Kamisato/internal/errwrap"
	"github.com/Hayao0819/Kamisato/pkg/pacman/keyring"
)

func filesCmd() *cobra.Command {
	var (
		name      string
		outputDir string
		revoked   []string
	)
	cmd := &cobra.Command{
		Use:   "files",
		Short: "Write the keyring files (<name>.gpg, -trusted, -revoked) into a directory",
		Long:  "Regenerate just the three pacman keyring files from the managed key into a directory. This suits an existing keyring source repo that keeps its own Makefile/PKGBUILD and install hook: ayaka owns the key material, the repo owns packaging and versioning.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			k, _, err := shared.LoadSigningKey(cmd)
			if err != nil {
				return err
			}
			pub, err := k.PublicEntity()
			if err != nil {
				return errwrap.WrapErr(err, "export public key")
			}
			files, err := keyring.BuildFiles(name, []*openpgp.Entity{pub}, []string{k.PrimaryFingerprint()}, revoked)
			if err != nil {
				return errwrap.WrapErr(err, "build keyring files")
			}
			if err := files.Write(outputDir); err != nil {
				return errwrap.WrapErr(err, "write keyring files")
			}
			out := cmd.OutOrStdout()
			for _, suffix := range []string{".gpg", "-trusted", "-revoked"} {
				fmt.Fprintln(out, filepath.Join(outputDir, name+suffix))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "Keyring identifier (the <name>.gpg stem) (required)")
	cmd.Flags().StringVar(&outputDir, "output-dir", "", "Directory to write the three files into (required)")
	cmd.Flags().StringSliceVar(&revoked, "revoked", nil, "Extra primary fingerprints to list as revoked (repeatable)")
	_ = cmd.MarkFlagRequired("name")
	_ = cmd.MarkFlagRequired("output-dir")
	return cmd
}
