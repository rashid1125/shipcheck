package main

import (
	"os"

	"github.com/rashid1125/shipcheck/cmd"
)

func main() {
	os.Exit(cmd.Run(os.Args[1:], os.Stdout, os.Stderr))
}
