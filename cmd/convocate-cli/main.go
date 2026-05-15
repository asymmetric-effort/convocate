package main

import (
	"fmt"
	"os"

	"github.com/asymmetric-effort/convocate/internal/cli"
)

// Version is set at build time via -ldflags.
var Version = "dev"

func main() {
	if len(os.Args) > 1 && os.Args[1] == "--version" {
		fmt.Println(Version)
		return
	}
	fmt.Fprintf(os.Stderr, "convocate-cli %s\n", Version)
	os.Exit(cli.Run(os.Args[1:]))
}
