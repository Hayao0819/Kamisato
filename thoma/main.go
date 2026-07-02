package main

import (
	"fmt"
	"os"

	"github.com/Hayao0819/Kamisato/thoma/cmd"
)

func main() {
	if err := cmd.RootCmd().Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "thoma: %v\n", err)
		os.Exit(1)
	}
}
