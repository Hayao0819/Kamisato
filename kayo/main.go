package main

import (
	"os"

	"github.com/Hayao0819/Kamisato/kayo/cmd"
)

func main() {
	if err := cmd.RootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}
