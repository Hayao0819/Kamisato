package main

import (
	"os"

	"github.com/Hayao0819/Kamisato/ayato/cmd"
	cmdline "github.com/Hayao0819/Kamisato/internal/cli"
)

func main() {
	os.Exit(cmdline.Execute(cmd.RootCmd()))
}
