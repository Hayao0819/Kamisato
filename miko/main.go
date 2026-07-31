package main

import (
	"os"

	"github.com/Hayao0819/Kamisato/internal/cliutil"
	"github.com/Hayao0819/Kamisato/miko/cmd"
)

func main() {
	os.Exit(cliutil.Execute(cmd.RootCmd()))
}
