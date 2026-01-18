package main

import (
	"github.com/Kong/kwot/cmd"
)

// Version will be set during build via ldflags
var Version = "dev"

func init() {
	cmd.SetVersion(Version)
}

func main() {
	cmd.Execute()
}
