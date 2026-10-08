// Package keyringcmd implements `ayaka keyring`: it turns the public half of the
// repository signing key (managed by `ayaka key`) into a distributable pacman
// keyring package and publishes it. The private key never leaves the local
// machine; only public key material is packaged.
package keyringcmd

import (
	"github.com/Hayao0819/Kamisato/ayaka/cmd/internal/keyinput"
	bootstrapcmd "github.com/Hayao0819/Kamisato/ayaka/cmd/keyring/bootstrap"
	buildcmd "github.com/Hayao0819/Kamisato/ayaka/cmd/keyring/build"
	filescmd "github.com/Hayao0819/Kamisato/ayaka/cmd/keyring/files"
	publishcmd "github.com/Hayao0819/Kamisato/ayaka/cmd/keyring/publish"
	"github.com/spf13/cobra"
)

// Cmd builds the `ayaka keyring` command group.
func Cmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "keyring",
		Short: "Build and publish the repository keyring package",
		Long:  "Package the signing key's public half as a pacman keyring (the <name>.gpg + -trusted + -revoked files plus a populate hook) and publish it to the repository so users can trust it.",
	}
	keyinput.AddKeyFlags(cmd)
	cmd.AddCommand(
		buildcmd.Cmd(),
		filescmd.Cmd(),
		publishcmd.Cmd(),
		bootstrapcmd.Cmd(),
	)
	return cmd
}
