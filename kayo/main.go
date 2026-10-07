package main

import (
	"os"

	cmdline "github.com/Hayao0819/Kamisato/internal/cli"
	"github.com/Hayao0819/Kamisato/kayo/cmd"
)

func main() {
	os.Exit(cmdline.Execute(cmd.RootCmd()))
}
