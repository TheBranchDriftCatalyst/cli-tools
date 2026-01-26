package main

import (
	"os"

	"github.com/TheBranchDriftCatalyst/dotfiles-2024/cli/wipctl/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}