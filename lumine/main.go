package main

import (
	"os"

	"github.com/Hayao0819/Kamisato/internal/cliutil"
	"github.com/Hayao0819/Kamisato/lumine/cmd"
)

//go:generate go run -C web github.com/gzuidhof/tygo@latest generate

func main() {
	os.Exit(cliutil.Execute(cmd.RootCmd()))
}
