package main

import (
	"os"

	"github.com/Hayao0819/Kamisato/lumine/cmd"
)

//go:generate go run -C web github.com/gzuidhof/tygo@latest generate

func main() {
	if err := cmd.RootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}
