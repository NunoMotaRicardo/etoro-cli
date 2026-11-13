// Command etoro-cli is a token-lean client for the eToro Public API.
package main

import (
	"fmt"
	"os"

	"github.com/NunoMotaRicardo/etoro-cli/internal/cli"
)

func main() {
	app, err := cli.NewApp()
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	os.Exit(cli.Run(app, os.Args[1:]))
}
