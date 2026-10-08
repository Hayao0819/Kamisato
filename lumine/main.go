package main

import (
	"os"

	cmdline "github.com/Hayao0819/Kamisato/internal/cli"
	"github.com/Hayao0819/Kamisato/lumine/cmd"
)

//go:generate go -C web tool tygo generate

func main() {
	os.Exit(cmdline.Execute(cmd.RootCmd()))
}
