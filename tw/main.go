package main

import (
	"monorepo/twigg/cli"
	"monorepo/twiggbuildflags"
	"os"
)

func main() {
	// Set the arguments to debug here:
	// os.Args = []string{"ag.bin (fake executable name)", "commit", "test"}
	os.Exit(cli.Run(nil, twiggbuildflags.TwiggWebUrl, nil, nil))
}
