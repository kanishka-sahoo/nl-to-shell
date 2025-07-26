package main

import (
	"fmt"
	"os"

	"github.com/nl-to-shell/nl-to-shell/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
