package main

import (
	"os"

	"github.com/TheBranchDriftCatalyst/cli-tools/cmd/wipctl/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}