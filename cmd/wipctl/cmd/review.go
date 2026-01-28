package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/TheBranchDriftCatalyst/cli-tools/cmd/wipctl/internal/ai"
	"github.com/TheBranchDriftCatalyst/cli-tools/cmd/wipctl/internal/gitexec"
	"github.com/TheBranchDriftCatalyst/cli-tools/cmd/wipctl/internal/status"
	"github.com/TheBranchDriftCatalyst/cli-tools/cmd/wipctl/internal/ui"
	"github.com/TheBranchDriftCatalyst/cli-tools/cmd/wipctl/internal/workspace"
)

var reviewCmd = &cobra.Command{
	Use:   "review [repository-path]",
	Short: "AI-powered workspace context briefing for future work sessions",
	Long: `Generate an intelligent workspace briefing for when you return to work.

Analyzes your workspace state across all repositories and provides:
- What you were working on in each repo
- Current state and changes made
- Context about ongoing work
- Suggested next steps and priorities
- Where to pick up development

This is perfect for multi-repo workflows where you need to remember:
- Which repos have changes
- What the changes are about
- What you were in the middle of

Configure AI provider with environment variables:
  WIPCTL_AI_PROVIDER=claude|openai|ollama
  WIPCTL_AI_TOKEN=your-api-key
  WIPCTL_AI_MODEL=claude-3-haiku-20240307

Examples:
  wipctl review                    # Review entire workspace context
  wipctl review /path/to/repo      # Review specific repository context`,
	RunE: runReview,
}

func init() {
	rootCmd.AddCommand(reviewCmd)
}

func runReview(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Check if specific repo is requested or workspace-wide review
	if len(args) > 0 {
		return reviewSingleRepository(ctx, args[0])
	}

	return reviewWorkspaceContext(ctx)
}

func reviewSingleRepository(ctx context.Context, repoPath string) error {
	// Validate it's a git repository
	if !isGitRepository(repoPath) {
		ui.Error("Not a git repository: " + repoPath)
		return fmt.Errorf("not a git repository")
	}

	ui.Info("🔍 Analyzing repository context...")

	// Get repository status
	status, err := gitexec.Status(ctx, repoPath)
	if err != nil {
		ui.Error("Failed to get repository status: " + err.Error())
		return err
	}

	// Get additional git information for context review
	reviewInput, err := buildWorkspaceContextInput(ctx, repoPath, status)
	if err != nil {
		ui.Error("Failed to collect context data: " + err.Error())
		return err
	}

	ui.Info("🤖 Generating workspace context briefing...")

	// Get AI generator
	aiConfig := ai.LoadConfigFromEnv()
	generator := ai.NewGenerator(aiConfig)

	// Convert to WorkspaceContextInput for enhanced briefing
	workspaceInput := buildSingleRepoWorkspaceInput(*reviewInput)
	review, err := generator.WorkspaceContext(ctx, workspaceInput)
	if err != nil {
		ui.Error("Failed to generate context briefing: " + err.Error())
		return err
	}

	// Display the context briefing
	ui.Success("📋 Workspace Context Briefing")
	ui.Info("──────────────────────────────")
	ui.CyberpunkBanner("WORK SESSION CONTEXT")

	println(review)

	return nil
}

func reviewWorkspaceContext(ctx context.Context) error {
	ui.Info("🔍 Analyzing workspace context across all repositories...")

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

	ui.Info(fmt.Sprintf("📊 Collecting context from %d repositories...", len(repos)))

	// Collect status from all repositories
	collector := status.NewCollector(8)
	results, err := collector.CollectStatus(ctx, repos)
	if err != nil {
		ui.Error("Failed to collect repository status: " + err.Error())
		return err
	}

	ui.Info("🤖 Generating comprehensive workspace briefing...")

	// Get AI generator
	aiConfig := ai.LoadConfigFromEnv()
	generator := ai.NewGenerator(aiConfig)

	// Convert to enhanced WorkspaceContextInput
	workspaceInput := buildEnhancedWorkspaceInput(results)
	briefing, err := generator.WorkspaceContext(ctx, workspaceInput)
	if err != nil {
		ui.Error("Failed to generate workspace briefing: " + err.Error())
		return err
	}

	// Display the workspace briefing
	ui.Success("📋 Multi-Repository Workspace Context")
	ui.Info("─────────────────────────────────────────")
	ui.CyberpunkBanner("WORKSPACE SESSION BRIEFING")

	println(briefing)

	return nil
}

func buildWorkspaceContextInput(ctx context.Context, repoPath string, status *gitexec.RepoStatus) (*ai.PRReviewInput, error) {
	// Get diff stat for context
	diffStat, err := gitexec.DiffStatCached(ctx, repoPath)
	if err != nil {
		diffStat = ""
	}

	// Get name status for file changes
	nameStatus, err := gitexec.DiffNameStatusCached(ctx, repoPath)
	if err != nil {
		nameStatus = ""
	}

	// Get recent commit messages for work context
	commitMsgs, err := gitexec.LogNSubjects(ctx, repoPath, 10)
	if err != nil {
		commitMsgs = []string{}
	}

	// Get repository name
	repoName := filepath.Base(repoPath)
	if repoName == "." {
		if absPath, err := filepath.Abs(repoPath); err == nil {
			repoName = filepath.Base(absPath)
		}
	}

	return &ai.PRReviewInput{
		Repo:         repoName,
		Branch:       status.Branch,
		DiffStat:     diffStat,
		NameStatus:   nameStatus,
		CommitMsgs:   commitMsgs,
		FilesCount:   status.FilesChanged,
		LinesAdded:   status.LinesAdded,
		LinesRemoved: status.LinesRemoved,
	}, nil
}

//nolint:unused // TODO: will be used for workspace-wide reviews
func buildWorkspaceReviewInput(results map[string]*gitexec.RepoStatus) ai.SynopsisInput {
	var repositories []ai.RepoSummary
	totalFiles := 0
	totalLines := 0
	totalCommits := 0

	for repoName, status := range results {
		if status.Error != "" {
			continue // Skip errored repos
		}

		repoSummary := ai.RepoSummary{
			Name:         repoName,
			Branch:       status.Branch,
			Status:       getWorkspaceStatusString(status),
			FilesChanged: status.FilesChanged,
			LinesAdded:   status.LinesAdded,
			LinesRemoved: status.LinesRemoved,
			Commits:      status.Commits,
		}

		repositories = append(repositories, repoSummary)
		totalFiles += status.FilesChanged
		totalLines += status.LinesAdded + status.LinesRemoved
		totalCommits += status.Commits
	}

	return ai.SynopsisInput{
		Repositories: repositories,
		TotalFiles:   totalFiles,
		TotalLines:   totalLines,
		TotalCommits: totalCommits,
	}
}

func buildSingleRepoWorkspaceInput(reviewInput ai.PRReviewInput) ai.WorkspaceContextInput {
	recentWork := reviewInput.CommitMsgs
	if len(recentWork) > 5 {
		recentWork = recentWork[:5] // Limit to 5 most recent
	}

	workspaceRepo := ai.WorkspaceRepo{
		Name:         reviewInput.Repo,
		Branch:       reviewInput.Branch,
		Status:       "dirty", // Single repo review implies changes
		FilesChanged: reviewInput.FilesCount,
		LinesAdded:   reviewInput.LinesAdded,
		LinesRemoved: reviewInput.LinesRemoved,
		Commits:      len(reviewInput.CommitMsgs),
		RecentWork:   recentWork,
		Changes:      reviewInput.NameStatus,
	}

	activeRepos := 1
	dirtyRepos := 1
	if reviewInput.FilesCount == 0 {
		dirtyRepos = 0
		workspaceRepo.Status = "clean"
	}

	return ai.WorkspaceContextInput{
		Repositories: []ai.WorkspaceRepo{workspaceRepo},
		TotalFiles:   reviewInput.FilesCount,
		TotalLines:   reviewInput.LinesAdded + reviewInput.LinesRemoved,
		TotalCommits: len(reviewInput.CommitMsgs),
		ActiveRepos:  activeRepos,
		DirtyRepos:   dirtyRepos,
	}
}

func buildEnhancedWorkspaceInput(results map[string]*gitexec.RepoStatus) ai.WorkspaceContextInput {
	var repositories []ai.WorkspaceRepo
	totalFiles := 0
	totalLines := 0
	totalCommits := 0
	activeRepos := 0
	dirtyRepos := 0

	for repoName, status := range results {
		if status.Error != "" {
			continue // Skip errored repos
		}

		// Get additional context for this repo
		ctx := context.Background()
		recentWork := []string{}
		changes := ""

		// Try to get recent commit messages
		if commitMsgs, err := gitexec.LogNSubjects(ctx, repoName, 5); err == nil {
			recentWork = commitMsgs
		}

		// Try to get name status for changes
		if nameStatus, err := gitexec.DiffNameStatusCached(ctx, repoName); err == nil && nameStatus != "" {
			changes = nameStatus
		}

		workspaceRepo := ai.WorkspaceRepo{
			Name:         repoName,
			Branch:       status.Branch,
			Status:       getWorkspaceStatusString(status),
			FilesChanged: status.FilesChanged,
			LinesAdded:   status.LinesAdded,
			LinesRemoved: status.LinesRemoved,
			Commits:      status.Commits,
			RecentWork:   recentWork,
			Changes:      changes,
		}

		repositories = append(repositories, workspaceRepo)
		totalFiles += status.FilesChanged
		totalLines += status.LinesAdded + status.LinesRemoved
		totalCommits += status.Commits

		if status.FilesChanged > 0 || status.Commits > 0 {
			activeRepos++
		}
		if status.Dirty > 0 {
			dirtyRepos++
		}
	}

	return ai.WorkspaceContextInput{
		Repositories: repositories,
		TotalFiles:   totalFiles,
		TotalLines:   totalLines,
		TotalCommits: totalCommits,
		ActiveRepos:  activeRepos,
		DirtyRepos:   dirtyRepos,
	}
}

func getWorkspaceStatusString(status *gitexec.RepoStatus) string {
	if !status.HasOrigin {
		return "no-origin"
	}
	if status.InProgress {
		return "in-progress"
	}
	if status.Dirty > 0 {
		return "dirty"
	}
	return "clean"
}

func isGitRepository(path string) bool {
	gitDir := filepath.Join(path, ".git")
	if info, err := os.Stat(gitDir); err == nil {
		return info.IsDir() || isGitFile(gitDir)
	}
	return false
}

func isGitFile(path string) bool {
	content, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	return len(content) > 8 && strings.HasPrefix(string(content), "gitdir: ")
}