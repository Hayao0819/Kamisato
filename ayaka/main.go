package main

import (
	"os"

	"github.com/Hayao0819/Kamisato/ayaka/cmd"
)

func main() {
	err := cmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}
