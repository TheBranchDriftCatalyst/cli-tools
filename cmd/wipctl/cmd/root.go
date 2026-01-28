package cmd

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/spf13/cobra"
	"github.com/TheBranchDriftCatalyst/cli-tools/cmd/wipctl/internal/ui"
)

var (
	workspacePath string
	reportDir     string
	hostName      string
	dryRun        bool
)

var rootCmd = &cobra.Command{
	Use:   "wipctl",
	Short: "Workspace-wide Git WIP Sync CLI",
	Long: `wipctl - Fast, safe, interactive CLI to synchronize many Git repos across machines
using timestamped WIP branches with AI-generated commit messages.

Features:
- status | push | pull | report subcommands
- Interactive staging prompts (or --auto-add)
- Safe pulls (auto-stash, conflict detection/report)
- Parallel repo processing with concurrency limits
- AI-generated commit messages via pluggable providers
- Markdown reports per run`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		initLogging()
	},
}

func Execute() error {
	ui.Banner("wipctl - Workspace Git WIP Sync")
	return rootCmd.Execute()
}

func init() {
	hostname, _ := os.Hostname()

	rootCmd.PersistentFlags().StringVarP(&workspacePath, "workspace", "w", ".", "workspace directory to search for Git repos")
	rootCmd.PersistentFlags().StringVar(&reportDir, "report-dir", "", "directory for reports (default: <workspace>/.wipctl)")
	rootCmd.PersistentFlags().StringVar(&hostName, "host", hostname, "host identifier for WIP branches")
	rootCmd.PersistentFlags().BoolVar(&dryRun, "dry-run", false, "show what would be done without making changes")
}

func initLogging() {
	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	if reportDir == "" {
		reportDir = fmt.Sprintf("%s/.wipctl", workspacePath)
	}
}