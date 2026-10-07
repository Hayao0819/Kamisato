package main

import (
	"os"

	cmdline "github.com/Hayao0819/Kamisato/internal/cli"
	"github.com/Hayao0819/Kamisato/lumine/cmd"
)

//go:generate go run -C web github.com/gzuidhof/tygo@latest generate

func main() {
	os.Exit(cmdline.Execute(cmd.RootCmd()))
}
