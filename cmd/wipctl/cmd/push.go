package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/spf13/cobra"
	"github.com/TheBranchDriftCatalyst/cli-tools/cmd/wipctl/internal/ai"
	"github.com/TheBranchDriftCatalyst/cli-tools/cmd/wipctl/internal/gitexec"
	"github.com/TheBranchDriftCatalyst/cli-tools/cmd/wipctl/internal/report"
	"github.com/TheBranchDriftCatalyst/cli-tools/cmd/wipctl/internal/ui"
	"github.com/TheBranchDriftCatalyst/cli-tools/cmd/wipctl/internal/workspace"
)

var (
	pushConcurrency int
	wipPrefix       string
	autoAdd         bool

	aiCommit     bool
	aiProvider   string
	aiEndpoint   string
	aiModel      string
	aiToken      string
	aiExec       string
	aiMaxTokens  int
	aiTemp       float64
	aiReview     bool
)

var pushCmd = &cobra.Command{
	Use:   "push",
	Short: "Create WIP branches and push to origin with optional AI-generated commits",
	Long: `Create WIP (Work In Progress) branches across all repositories and push them to origin.

For each repository that has changes:
1. Check preconditions (has origin, not in rebase/merge)
2. Fetch from origin
3. Handle dirty/untracked files (prompt or --auto-add)
4. Generate commit message (AI or fallback)
5. Create WIP branch and commit
6. Push WIP branch to origin
7. Push current branch if it exists on origin

WIP branches use the format: wip/<host>/<timestamp>`,
	RunE: runPush,
}

func init() {
	rootCmd.AddCommand(pushCmd)

	pushCmd.Flags().IntVar(&pushConcurrency, "concurrency", 6, "number of concurrent repository operations")
	pushCmd.Flags().StringVar(&wipPrefix, "prefix", "", "WIP branch prefix (default: wip/<host>/<timestamp>)")
	pushCmd.Flags().BoolVar(&autoAdd, "auto-add", false, "automatically add all changes without prompting")

	pushCmd.Flags().BoolVar(&aiCommit, "ai-commit", false, "use AI to generate commit messages")
	pushCmd.Flags().StringVar(&aiProvider, "ai-provider", "none", "AI provider: none|exec|openai|ollama")
	pushCmd.Flags().StringVar(&aiEndpoint, "ai-endpoint", "", "AI endpoint URL")
	pushCmd.Flags().StringVar(&aiModel, "ai-model", "", "AI model name")
	pushCmd.Flags().StringVar(&aiToken, "ai-token", "", "AI API token")
	pushCmd.Flags().StringVar(&aiExec, "ai-exec", "", "path to external AI executable")
	pushCmd.Flags().IntVar(&aiMaxTokens, "ai-max-tokens", 256, "AI max tokens")
	pushCmd.Flags().Float64Var(&aiTemp, "ai-temperature", 0.1, "AI temperature")
	pushCmd.Flags().BoolVar(&aiReview, "ai-review", false, "review AI-generated messages (forces concurrency=1)")
}

func runPush(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	if dryRun {
		ctx = context.WithValue(ctx, gitexec.DryRunKey, true)
		ui.Info("🧪 DRY RUN MODE - No actual git operations will be performed")
	}

	if aiReview {
		pushConcurrency = 1
		ui.Info("AI review enabled - using serial processing")
	}

	if wipPrefix == "" {
		wipPrefix = fmt.Sprintf("wip/%s/%s", hostName, time.Now().Format("20060102-150405"))
	}

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

	ui.Info(fmt.Sprintf("Processing %d repositories with WIP prefix: %s", len(repos), wipPrefix))

	aiConfig := buildAIConfig()
	generator := ai.NewGenerator(aiConfig)

	rep := report.NewReport("WIP Push Report", workspacePath, reportDir, "push")

	var mu sync.Mutex
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, pushConcurrency)

	for _, repo := range repos {
		wg.Add(1)
		go func(repo workspace.Repo) {
			defer wg.Done()

			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			entry := processRepoPush(ctx, repo, generator, wipPrefix)

			mu.Lock()
			rep.AddEntry(entry)
			mu.Unlock()
		}(repo)
	}

	wg.Wait()

	if err := rep.Save(); err != nil {
		ui.Warning("Failed to save report: " + err.Error())
	}

	ui.Success("Push operation completed. Report saved.")
	return nil
}

func processRepoPush(ctx context.Context, repo workspace.Repo, generator ai.Generator, wipPrefix string) report.ReportEntry {
	entry := report.CreatePushEntry(repo.Name, "", wipPrefix, "")

	slog.Info("Processing repository", "repo", repo.Path)

	ok, reason := gitexec.Preconditions(ctx, repo.Path)
	if !ok {
		entry.Outcome = "skipped"
		entry.AddWarning(reason)
		ui.Warning(fmt.Sprintf("%s: %s", repo.Name, reason))
		return entry
	}

	if err := gitexec.Fetch(ctx, repo.Path); err != nil {
		entry.Outcome = "error"
		entry.AddError(fmt.Sprintf("fetch failed: %v", err))
		ui.Error(fmt.Sprintf("%s: fetch failed", repo.Name))
		return entry
	}

	status, err := gitexec.Status(ctx, repo.Path)
	if err != nil {
		entry.Outcome = "error"
		entry.AddError(fmt.Sprintf("status check failed: %v", err))
		return entry
	}

	entry.Details = fmt.Sprintf("%s (wip=%s)", status.Branch, wipPrefix)

	if status.Dirty > 0 || status.Untracked > 0 {
		hasJunk, junkFiles, err := gitexec.HasJunkFiles(ctx, repo.Path)
		if err != nil {
			entry.Outcome = "error"
			entry.AddError(fmt.Sprintf("junk check failed: %v", err))
			return entry
		}

		if hasJunk {
			entry.Outcome = "skipped"
			entry.AddWarning(fmt.Sprintf("untracked junk files found: %v", junkFiles))
			ui.Warning(fmt.Sprintf("%s: junk files found, fix .gitignore", repo.Name))
			return entry
		}

		if !autoAdd {
			if pushConcurrency == 1 {
				if !ui.Confirm(fmt.Sprintf("%s: Add all changes and continue?", repo.Name)) {
					entry.Outcome = "skipped"
					entry.AddWarning("user declined to add changes")
					return entry
				}
			} else {
				entry.Outcome = "skipped"
				entry.AddWarning("changes present but auto-add not enabled")
				return entry
			}
		}

		if err := gitexec.AddAll(ctx, repo.Path); err != nil {
			entry.Outcome = "error"
			entry.AddError(fmt.Sprintf("add all failed: %v", err))
			return entry
		}
	}

	message := generateCommitMessage(ctx, repo, generator, status)

	if err := gitexec.SwitchCreate(ctx, repo.Path, wipPrefix); err != nil {
		entry.Outcome = "error"
		entry.AddError(fmt.Sprintf("create WIP branch failed: %v", err))
		return entry
	}

	if err := gitexec.CommitAllowEmpty(ctx, repo.Path, message); err != nil {
		entry.Outcome = "error"
		entry.AddError(fmt.Sprintf("commit failed: %v", err))
		return entry
	}

	if err := gitexec.PushUpstream(ctx, repo.Path, wipPrefix); err != nil {
		entry.Outcome = "error"
		entry.AddError(fmt.Sprintf("push WIP branch failed: %v", err))
		return entry
	}

	hasRemote, err := gitexec.RemoteHasBranch(ctx, repo.Path, status.Branch)
	if err == nil && hasRemote {
		if err := gitexec.Switch(ctx, repo.Path, status.Branch); err != nil {
			entry.AddWarning(fmt.Sprintf("failed to switch back to %s", status.Branch))
		} else {
			if err := gitexec.Push(ctx, repo.Path, status.Branch); err != nil {
				entry.AddWarning(fmt.Sprintf("failed to push current branch %s", status.Branch))
			}
		}
	}

	entry.Outcome = "success"
	ui.Success(fmt.Sprintf("%s: WIP branch created and pushed", repo.Name))
	return entry
}

func generateCommitMessage(ctx context.Context, repo workspace.Repo, generator ai.Generator, status *gitexec.RepoStatus) string {
	fallback := fmt.Sprintf("chore(wip): checkpoint %s (%s) — %d files @ %s",
		hostName, status.Branch, status.Dirty+status.Untracked, time.Now().Format("2006-01-02 15:04:05"))

	if !aiCommit {
		return fallback
	}

	input := ai.CommitMsgInput{
		Repo:   repo.Name,
		Branch: status.Branch,
		Host:   hostName,
	}

	nameStatus, err := gitexec.DiffNameStatusCached(ctx, repo.Path)
	if err == nil {
		input.NameStatus = nameStatus
	}

	diffStat, err := gitexec.DiffStatCached(ctx, repo.Path)
	if err == nil {
		input.DiffStat = diffStat
	}

	untracked, err := gitexec.ListUntracked(ctx, repo.Path)
	if err == nil {
		input.Untracked = untracked
	}

	subjects, err := gitexec.LogNSubjects(ctx, repo.Path, 5)
	if err == nil {
		input.PriorSubjects = subjects
	}

	message, err := generator.CommitMessage(ctx, input)
	if err != nil {
		slog.Warn("AI commit message generation failed, using fallback",
			"repo", repo.Path,
			"error", err)
		return fallback
	}

	if message == "" {
		return fallback
	}

	if aiReview {
		ui.Info("AI-generated commit message:")
		fmt.Printf("  %s\n\n", message)
		if !ui.Confirm("Accept this message?") {
			return fallback
		}
	}

	return message
}

func buildAIConfig() ai.Config {
	envConfig := ai.LoadConfigFromEnv()

	config := ai.Config{
		Provider:    aiProvider,
		Endpoint:    aiEndpoint,
		Model:       aiModel,
		Token:       aiToken,
		ExecPath:    aiExec,
		MaxTokens:   aiMaxTokens,
		Temperature: aiTemp,
	}

	if config.Provider == "" && envConfig.Provider != "" {
		config.Provider = envConfig.Provider
	}
	if config.Endpoint == "" && envConfig.Endpoint != "" {
		config.Endpoint = envConfig.Endpoint
	}
	if config.Model == "" && envConfig.Model != "" {
		config.Model = envConfig.Model
	}
	if config.Token == "" && envConfig.Token != "" {
		config.Token = envConfig.Token
	}
	if config.ExecPath == "" && envConfig.ExecPath != "" {
		config.ExecPath = envConfig.ExecPath
	}

	return config
}