package main

import (
	"os"

	"github.com/Hayao0819/Kamisato/ayaka/cmd"
	"github.com/Hayao0819/Kamisato/internal/cliutil"
)

func main() {
	os.Exit(cliutil.Execute(cmd.RootCmd()))
}
