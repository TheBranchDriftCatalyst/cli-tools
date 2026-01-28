package cmd

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/TheBranchDriftCatalyst/cli-tools/cmd/wipctl/internal/ai"
	"github.com/TheBranchDriftCatalyst/cli-tools/cmd/wipctl/internal/gitexec"
	"github.com/TheBranchDriftCatalyst/cli-tools/cmd/wipctl/internal/report"
	"github.com/TheBranchDriftCatalyst/cli-tools/cmd/wipctl/internal/status"
	"github.com/TheBranchDriftCatalyst/cli-tools/cmd/wipctl/internal/ui"
	"github.com/TheBranchDriftCatalyst/cli-tools/cmd/wipctl/internal/workspace"
)

var checkpointCmd = &cobra.Command{
	Use:   "checkpoint",
	Short: "🚀 Hackerspeed checkpoint - analyze, commit, and push all repos",
	Long: `Create intelligent WIP checkpoint commits across all repositories at hackerspeed.

This command performs a full workflow for every repository with changes:
- Analyzes current state and detects changes
- Auto-stages all modifications (no prompts)
- Generates AI-powered commit messages
- Creates timestamped WIP branch
- Pushes to remote with full sync

Perfect for rapid development cycles and end-of-day checkpoints.

Configure AI provider with environment variables:
  WIPCTL_AI_PROVIDER=claude|openai|ollama
  WIPCTL_AI_TOKEN=your-api-key
  WIPCTL_AI_MODEL=claude-3-haiku-20240307

Examples:
  wipctl checkpoint                    # Checkpoint all repos with AI commits
  wipctl checkpoint --dry-run          # Preview what would be checkpointed
  wipctl checkpoint --message="EOD"    # Use custom message prefix`,
	RunE: runCheckpoint,
}

var (
	checkpointMessage     string
	checkpointConcurrency int
	checkpointFeature     string
	checkpointCrossRepo   bool
)

func init() {
	rootCmd.AddCommand(checkpointCmd)

	checkpointCmd.Flags().StringVar(&checkpointMessage, "message", "", "Custom message prefix for commits")
	checkpointCmd.Flags().IntVar(&checkpointConcurrency, "concurrency", 8, "Number of parallel operations")
	checkpointCmd.Flags().StringVar(&checkpointFeature, "feature", "", "Cross-repo feature name for coordinated commits")
	checkpointCmd.Flags().BoolVar(&checkpointCrossRepo, "cross-repo", false, "Enable cross-repository feature coordination")
}

func runCheckpoint(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Add dry-run context if needed
	if dryRun {
		ctx = context.WithValue(ctx, gitexec.DryRunKey, true)
	}

	ui.CyberpunkBanner("HACKERSPEED CHECKPOINT")
	ui.Info("🚀 Initiating rapid workspace checkpoint...")

	// Discover all repositories
	repos, err := workspace.Discover(ctx, workspacePath)
	if err != nil {
		ui.Error("Failed to discover repositories: " + err.Error())
		return err
	}

	if len(repos) == 0 {
		ui.Warning("No Git repositories found in workspace")
		return nil
	}

	ui.Info(fmt.Sprintf("⚡ Found %d repositories - analyzing at hackerspeed...", len(repos)))

	// Collect status from all repositories
	collector := status.NewCollector(checkpointConcurrency)
	results, err := collector.CollectStatus(ctx, repos)
	if err != nil {
		ui.Error("Failed to collect repository status: " + err.Error())
		return err
	}

	// Filter repos that need checkpointing
	checkpointRepos := filterCheckpointCandidates(results)

	if len(checkpointRepos) == 0 {
		ui.Success("✨ All repositories are clean - no checkpoint needed!")
		return nil
	}

	ui.Info(fmt.Sprintf("💾 Checkpointing %d repositories with changes...", len(checkpointRepos)))

	// Validate cross-repo feature mode
	if checkpointCrossRepo && checkpointFeature == "" {
		ui.Error("Cross-repo mode requires --feature flag")
		return fmt.Errorf("--cross-repo requires --feature to be specified")
	}

	// Get AI generator for commit messages
	aiConfig := ai.LoadConfigFromEnv()
	generator := ai.NewGenerator(aiConfig)

	// Create enhanced checkpoint report
	checkpointReport := report.NewCheckpointReport("Hackerspeed Checkpoint", workspacePath, reportDir, checkpointFeature, checkpointCrossRepo)
	checkpointReport.TotalRepos = len(repos)

	if checkpointCrossRepo {
		ui.Info(fmt.Sprintf("🔗 Cross-repo feature mode: %s", checkpointFeature))
	}

	// Process each repository that needs checkpointing
	for _, repoPath := range checkpointRepos {
		repoStatus := results[repoPath]
		if repoStatus.Error != "" {
			continue // Skip errored repos
		}

		ui.Info(fmt.Sprintf("🔄 Checkpointing %s...", filepath.Base(repoPath)))

		entry := processEnhancedCheckpointRepo(ctx, repoPath, repoStatus, generator)
		checkpointReport.AddCheckpointEntry(entry)

		if entry.Outcome == "success" {
			ui.Success(fmt.Sprintf("✅ %s checkpointed", filepath.Base(repoPath)))
		} else {
			ui.Error(fmt.Sprintf("❌ %s failed: %s", filepath.Base(repoPath), entry.Details))
		}
	}

	// Generate workspace summary
	checkpointReport.GenerateWorkspaceSummary()

	// Save enhanced checkpoint report
	if err := checkpointReport.Save(); err != nil {
		ui.Warning("Failed to save checkpoint report: " + err.Error())
	}

	ui.Success("🚀 Hackerspeed checkpoint complete!")
	ui.Info(fmt.Sprintf("📋 Checkpointed %d repositories", len(checkpointRepos)))

	return nil
}

func filterCheckpointCandidates(results map[string]*gitexec.RepoStatus) []string {
	var candidates []string

	for repoPath, status := range results {
		if status.Error != "" {
			continue // Skip errored repos
		}

		// Include repos with any changes or commits ahead
		if status.Dirty > 0 || status.Untracked > 0 || status.Commits > 0 {
			candidates = append(candidates, repoPath)
		}
	}

	return candidates
}

func processEnhancedCheckpointRepo(ctx context.Context, repoPath string, status *gitexec.RepoStatus, generator ai.Generator) report.CheckpointEntry {
	repoName := filepath.Base(repoPath)

	// Create enhanced checkpoint entry
	entry := report.CreateCheckpointEntry(repoName, status.Branch, "processing", fmt.Sprintf("branch: %s", status.Branch))

	// Add feature coordination if enabled
	if checkpointFeature != "" {
		entry.FeatureName = checkpointFeature
	}
	if checkpointCrossRepo {
		entry.CrossRepoGroup = checkpointFeature
	}

	// Check preconditions
	if !status.HasOrigin {
		entry.Outcome = "skipped"
		entry.Details = "no origin remote"
		entry.AddWarning("Repository has no origin remote - skipping checkpoint")
		return entry
	}

	if status.InProgress {
		entry.Outcome = "skipped"
		entry.Details = "rebase/merge in progress"
		entry.AddWarning("Repository has ongoing rebase/merge - skipping checkpoint")
		return entry
	}

	// Collect detailed repo information before staging
	entry.FilesModified = status.Dirty
	entry.FilesAdded = status.Untracked
	entry.LinesAdded = status.LinesAdded
	entry.LinesRemoved = status.LinesRemoved

	// Get changed files list
	if nameStatus, err := gitexec.DiffNameStatusCached(ctx, repoPath); err == nil && nameStatus != "" {
		lines := strings.Split(nameStatus, "\n")
		for _, line := range lines {
			if strings.TrimSpace(line) != "" {
				parts := strings.SplitN(strings.TrimSpace(line), "\t", 2)
				if len(parts) == 2 {
					entry.ChangedFiles = append(entry.ChangedFiles, parts[1])
				}
			}
		}
	}

	// Get recent commits for context
	if recentCommits, err := gitexec.LogNSubjects(ctx, repoPath, 3); err == nil {
		entry.RecentCommits = recentCommits
	}

	// Stage all changes (hackerspeed = no prompts)
	if status.Dirty > 0 || status.Untracked > 0 {
		ui.Info(fmt.Sprintf("📦 Staging %d dirty + %d untracked files...", status.Dirty, status.Untracked))

		if err := gitexec.AddAll(ctx, repoPath); err != nil {
			entry.Outcome = "failed"
			entry.Details = "failed to stage changes"
			entry.AddError("git add failed: " + err.Error())
			return entry
		}
	}

	// Generate AI commit message with cross-repo context
	commitMsg, err := generateEnhancedCheckpointCommitMessage(ctx, repoPath, status, generator)
	if err != nil {
		ui.Warning("AI commit generation failed, using fallback: " + err.Error())
		commitMsg = generateFallbackCheckpointMessage(repoName, status)
	}
	entry.CommitMessage = commitMsg

	// Create checkpoint commit
	if err := gitexec.CommitAllowEmpty(ctx, repoPath, commitMsg); err != nil {
		entry.Outcome = "failed"
		entry.Details = "failed to create commit"
		entry.AddError("git commit failed: " + err.Error())
		return entry
	}

	// Try to get the commit hash
	if hash, err := gitexec.GetLastCommitHash(ctx, repoPath); err == nil {
		entry.CommitHash = hash[:8] // Short hash
	}

	// Generate WIP branch name (with feature coordination if enabled)
	timestamp := time.Now().Format("20060102-150405")
	var wipBranch string
	if checkpointFeature != "" {
		wipBranch = fmt.Sprintf("wip/%s/%s/%s", hostName, checkpointFeature, timestamp)
	} else {
		wipBranch = fmt.Sprintf("wip/%s/%s", hostName, timestamp)
	}
	entry.WipBranch = wipBranch

	// Create and push WIP branch
	if err := gitexec.SwitchCreate(ctx, repoPath, wipBranch); err != nil {
		entry.Outcome = "failed"
		entry.Details = "failed to create WIP branch"
		entry.AddError("git switch failed: " + err.Error())
		return entry
	}

	// Push WIP branch to origin
	if err := gitexec.PushUpstream(ctx, repoPath, wipBranch); err != nil {
		entry.Outcome = "failed"
		entry.Details = "failed to push WIP branch"
		entry.AddError("git push failed: " + err.Error())
		return entry
	}

	// Switch back to original branch
	if err := gitexec.Switch(ctx, repoPath, status.Branch); err != nil {
		entry.AddWarning("Failed to switch back to original branch: " + err.Error())
	}

	// Push original branch if it exists on origin
	hasRemoteBranch, err := gitexec.RemoteHasBranch(ctx, repoPath, status.Branch)
	if err == nil && hasRemoteBranch {
		if err := gitexec.Push(ctx, repoPath, status.Branch); err != nil {
			entry.AddWarning("Failed to push original branch: " + err.Error())
		}
	}

	entry.Outcome = "success"
	entry.Details = fmt.Sprintf("checkpointed to %s", wipBranch)

	return entry
}

func generateEnhancedCheckpointCommitMessage(ctx context.Context, repoPath string, status *gitexec.RepoStatus, generator ai.Generator) (string, error) {
	// Get diff information for AI
	diffStat, err := gitexec.DiffStatCached(ctx, repoPath)
	if err != nil {
		diffStat = ""
	}

	nameStatus, err := gitexec.DiffNameStatusCached(ctx, repoPath)
	if err != nil {
		nameStatus = ""
	}

	// Get recent commit subjects for context
	recentCommits, err := gitexec.LogNSubjects(ctx, repoPath, 5)
	if err != nil {
		recentCommits = []string{}
	}

	// Build commit message input
	input := ai.CommitMsgInput{
		Repo:          filepath.Base(repoPath),
		Branch:        status.Branch,
		Host:          hostName,
		NameStatus:    nameStatus,
		DiffStat:      diffStat,
		Untracked:     []string{}, // We auto-stage everything
		PriorSubjects: recentCommits,
	}

	// Add custom message prefix if provided
	message, err := generator.CommitMessage(ctx, input)
	if err != nil {
		return "", err
	}

	// Add cross-repo feature context
	if checkpointFeature != "" {
		message = fmt.Sprintf("feat(%s): %s", checkpointFeature, message)
	}

	if checkpointMessage != "" {
		message = fmt.Sprintf("[%s] %s", checkpointMessage, message)
	}

	return message, nil
}

func generateFallbackCheckpointMessage(repoName string, status *gitexec.RepoStatus) string {
	timestamp := time.Now().Format("15:04")

	message := fmt.Sprintf("checkpoint(%s): rapid development sync", timestamp)

	if checkpointMessage != "" {
		message = fmt.Sprintf("[%s] %s", checkpointMessage, message)
	}

	// Add some context about changes
	var details []string
	if status.Dirty > 0 {
		details = append(details, fmt.Sprintf("%d files modified", status.Dirty))
	}
	if status.Untracked > 0 {
		details = append(details, fmt.Sprintf("%d files added", status.Untracked))
	}

	if len(details) > 0 {
		message += " - " + strings.Join(details, ", ")
	}

	return message
}