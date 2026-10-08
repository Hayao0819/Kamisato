package buildflags

import (
	"github.com/Hayao0819/Kamisato/ayaka/keyring"
	"github.com/spf13/cobra"
)

// Add binds the service request directly; there is no second flags-only model.
func Add(command *cobra.Command, params *keyring.BuildParams) {
	flags := command.Flags()
	flags.StringVar(&params.Name, "name", "", "Keyring identifier (required)")
	flags.StringVar(&params.Version, "version", "", "Package version (default: today's date)")
	flags.StringVar(&params.Packager, "packager", "", "PKGINFO packager field")
	flags.StringVar(&params.Desc, "desc", "", "Package description (default: '<name> PGP keyring')")
	flags.StringSliceVar(&params.Revoked, "revoked", nil, "Extra revoked primary fingerprints (repeatable)")
	flags.StringSliceVar(&params.License, "license", nil, "Package licenses (repeatable)")
	flags.StringSliceVar(&params.Depends, "depends", nil, "Package dependencies (repeatable)")
	flags.BoolVar(&params.Sign, "sign", true, "Sign the built package with the signing key")
	_ = command.MarkFlagRequired("name")
}
