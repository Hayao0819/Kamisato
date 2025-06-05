package main

import (
	"fmt"
	"os"

	"github.com/Hayao0819/Kamisato/ayato/cmd"
)

// @title gin-swagger todos 
// @version 1.0
// @description このswaggerはgin-swaggerの見本apiです
func main() {
	if err := cmd.RootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
