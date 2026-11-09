package main

import (
	"os"

	"github.com/rashid1125/launchsafe/cmd"
)

func main() {
	os.Exit(cmd.Run(os.Args[1:], os.Stdout, os.Stderr))
}
