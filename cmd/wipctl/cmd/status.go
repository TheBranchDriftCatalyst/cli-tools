package cmd

import (
	"context"

	"github.com/spf13/cobra"
	"github.com/TheBranchDriftCatalyst/cli-tools/cmd/wipctl/internal/operations"
	"github.com/TheBranchDriftCatalyst/cli-tools/cmd/wipctl/internal/ui"
	"github.com/TheBranchDriftCatalyst/cli-tools/cmd/wipctl/internal/workspace"
)

var (
	statusConcurrency int
	statusWithAI      bool
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show status of all Git repositories in the workspace",
	Long: `Show a table with the status of all Git repositories found in the workspace.

Displays:
- Repository name
- Current branch
- Dirty files count
- Untracked files count
- Commits ahead of origin
- Commits behind origin

This command does not fetch from remotes to keep it fast.`,
	RunE: runStatus,
}

func init() {
	rootCmd.AddCommand(statusCmd)
	statusCmd.Flags().IntVar(&statusConcurrency, "concurrency", 8, "number of concurrent repository operations")
	statusCmd.Flags().BoolVar(&statusWithAI, "ai", false, "include AI-powered synopsis")
}

func runStatus(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Discover repositories
	ui.Info("Discovering Git repositories...")
	repos, err := workspace.Discover(ctx, workspacePath)
	if err != nil {
		ui.Error("Failed to discover repositories: " + err.Error())
		return err
	}

	if len(repos) == 0 {
		ui.Warning("No Git repositories found in workspace")
		return nil
	}

	// Create status handler with unified architecture
	handler := operations.NewStatusHandler(statusWithAI, statusConcurrency)

	// Process workspace status using streamlined architecture
	return handler.ProcessWorkspaceStatus(ctx, repos)
}

// All status display logic moved to operations/status.go for DRY architecture