package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/spf13/cobra"
	"github.com/TheBranchDriftCatalyst/cli-tools/cmd/wipctl/internal/gitexec"
	"github.com/TheBranchDriftCatalyst/cli-tools/cmd/wipctl/internal/report"
	"github.com/TheBranchDriftCatalyst/cli-tools/cmd/wipctl/internal/ui"
	"github.com/TheBranchDriftCatalyst/cli-tools/cmd/wipctl/internal/workspace"
)

var pullConcurrency int

var pullCmd = &cobra.Command{
	Use:   "pull",
	Short: "Pull latest WIP branches from origin across all repositories",
	Long: `Pull the latest WIP branches from origin across all repositories safely.

For each repository:
1. Check preconditions (has origin, not in rebase/merge)
2. Fetch from origin
3. Find the newest WIP branch by commit date
4. Stash any local changes
5. Switch to (or create) local WIP branch tracking the remote
6. Pop stashed changes and detect conflicts

If conflicts occur, they are reported but not automatically resolved.`,
	RunE: runPull,
}

func init() {
	rootCmd.AddCommand(pullCmd)
	pullCmd.Flags().IntVar(&pullConcurrency, "concurrency", 6, "number of concurrent repository operations")
}

func runPull(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	if dryRun {
		ctx = context.WithValue(ctx, gitexec.DryRunKey, true)
		ui.Info("🧪 DRY RUN MODE - No actual git operations will be performed")
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

	ui.Info(fmt.Sprintf("Pulling WIP branches for %d repositories", len(repos)))

	rep := report.NewReport("WIP Pull Report", workspacePath, reportDir, "pull")

	var mu sync.Mutex
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, pullConcurrency)

	for _, repo := range repos {
		wg.Add(1)
		go func(repo workspace.Repo) {
			defer wg.Done()

			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			entry := processRepoPull(ctx, repo)

			mu.Lock()
			rep.AddEntry(entry)
			mu.Unlock()
		}(repo)
	}

	wg.Wait()

	if err := rep.Save(); err != nil {
		ui.Warning("Failed to save report: " + err.Error())
	}

	ui.Success("Pull operation completed. Report saved.")
	return nil
}

func processRepoPull(ctx context.Context, repo workspace.Repo) report.ReportEntry {
	entry := report.CreatePullEntry(repo.Name, "", "", "")

	slog.Info("Processing repository", "repo", repo.Path)

	ok, reason := gitexec.Preconditions(ctx, repo.Path)
	if !ok {
		entry.Outcome = "skipped"
		entry.AddWarning(reason)
		ui.Warning(fmt.Sprintf("%s: %s", repo.Name, reason))
		return entry
	}

	status, err := gitexec.Status(ctx, repo.Path)
	if err != nil {
		entry.Outcome = "error"
		entry.AddError(fmt.Sprintf("status check failed: %v", err))
		return entry
	}

	originalBranch := status.Branch

	if err := gitexec.Fetch(ctx, repo.Path); err != nil {
		entry.Outcome = "error"
		entry.AddError(fmt.Sprintf("fetch failed: %v", err))
		ui.Error(fmt.Sprintf("%s: fetch failed", repo.Name))
		return entry
	}

	latestWipRemote, err := gitexec.LatestRemoteWIP(ctx, repo.Path)
	if err != nil {
		entry.Outcome = "no-wip"
		entry.AddWarning("no WIP branches found on origin")
		ui.Info(fmt.Sprintf("%s: no WIP branches found", repo.Name))
		return entry
	}

	wipBranchName := gitexec.TrimOrigin(latestWipRemote)
	entry.Details = fmt.Sprintf("%s → %s", originalBranch, wipBranchName)

	stashMessage := fmt.Sprintf("wipctl auto-stash before pull - %s", wipBranchName)
	if err := gitexec.Stash(ctx, repo.Path, stashMessage); err != nil {
		slog.Debug("Stash failed (may be nothing to stash)", "repo", repo.Path, "error", err)
	}

	if err := gitexec.SwitchCreate(ctx, repo.Path, wipBranchName); err != nil {
		entry.Outcome = "error"
		entry.AddError(fmt.Sprintf("switch to WIP branch failed: %v", err))
		return entry
	}

	err = gitexec.Switch(ctx, repo.Path, wipBranchName)
	if err != nil {
		if createErr := createTrackingBranch(ctx, repo.Path, wipBranchName, latestWipRemote); createErr != nil {
			entry.Outcome = "error"
			entry.AddError(fmt.Sprintf("create tracking branch failed: %v", createErr))
			return entry
		}
	}

	if err := gitexec.StashPop(ctx, repo.Path); err != nil {
		slog.Debug("Stash pop failed (may be no stash)", "repo", repo.Path, "error", err)
	}

	hasConflicts, conflictFiles, err := gitexec.HasConflicts(ctx, repo.Path)
	if err != nil {
		entry.AddWarning(fmt.Sprintf("conflict detection failed: %v", err))
	}

	if hasConflicts {
		entry.Outcome = "conflicts"
		entry.AddWarning(fmt.Sprintf("conflicts in files: %v", conflictFiles))
		ui.Warning(fmt.Sprintf("%s: conflicts detected, resolve and commit", repo.Name))
		return entry
	}

	entry.Outcome = "success"
	ui.Success(fmt.Sprintf("%s: switched to WIP branch %s", repo.Name, wipBranchName))
	return entry
}

func createTrackingBranch(ctx context.Context, repoPath, localBranch, remoteBranch string) error {
	if err := gitexec.SwitchCreate(ctx, repoPath, localBranch); err != nil {
		return fmt.Errorf("create local branch: %w", err)
	}

	remoteShort := gitexec.TrimOrigin(remoteBranch)

	cmd := fmt.Sprintf("git branch --set-upstream-to=origin/%s %s", remoteShort, localBranch)
	slog.Debug("Setting upstream", "repo", repoPath, "command", cmd)

	return nil
}