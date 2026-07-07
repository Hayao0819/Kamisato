// Package keycmd implements `ayaka key`: the lifecycle of the repository's own
// OpenPGP signing key (a primary plus a signing subkey). The private key stays on
// the local machine; only its public half is ever published, through a keyring
// package built by `ayaka keyring`.
package keycmd

import (
	"github.com/spf13/cobra"

	"github.com/Hayao0819/Kamisato/ayaka/cmd/shared"
)

// Cmd builds the `ayaka key` command group.
func Cmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "key",
		Short: "Manage the repository signing key",
		Long:  "Generate or import the OpenPGP key that signs this repository's packages, then maintain it: list its subkeys, export it, extend its validity, rotate or revoke subkeys, and revoke the whole key.",
	}
	shared.AddKeyFlags(cmd)
	cmd.AddCommand(
		generateCmd(),
		importCmd(),
		listCmd(),
		exportCmd(),
		expireCmd(),
		revokeCmd(),
		subkeyCmd(),
	)
	return cmd
}
