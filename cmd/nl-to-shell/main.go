package main

import (
	"fmt"
	"os"

	"github.com/nl-to-shell/nl-to-shell/internal/cli"
)

// exitFunc is a variable to allow mocking os.Exit in tests
var exitFunc = os.Exit

func main() {
	if err := cli.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		exitFunc(1)
	}
}
