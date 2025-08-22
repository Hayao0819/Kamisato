package cmd

import blinkycmd "github.com/Hayao0819/Kamisato/ayaka/cmd/blinky"

// Register the Blinky command as a subcommand
func init() {
	subCmds.Add(blinkycmd.Root())
}
