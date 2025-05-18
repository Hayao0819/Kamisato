package main

import (
	"fmt"
	"os"

	"github.com/Hayao0819/Kamisato/ayaka/cmd"
)

func main() {
	err := cmd.Execute()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %+v\n", err)
		os.Exit(1)
	}
}
