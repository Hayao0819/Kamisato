package main

import (
	"os"

	"github.com/Hayao0819/Kamisato/sara/cmd"
)

func main() {
	if err := cmd.RootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}
