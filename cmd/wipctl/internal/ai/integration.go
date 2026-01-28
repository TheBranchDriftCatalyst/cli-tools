package ai

import (
	"context"
	"fmt"

	"github.com/TheBranchDriftCatalyst/cli-tools/cmd/wipctl/internal/gitexec"
)

// Integration provides unified AI functionality across commands
type Integration struct {
	generator Generator
	enabled   bool
}

// NewIntegration creates a new AI integration with unified configuration loading
func NewIntegration() *Integration {
	config := LoadConfigFromEnv()

	// Only enable if we have a valid provider configured
	enabled := config.Provider != "" && config.Provider != "none"

	var generator Generator
	if enabled {
		generator = NewGenerator(config)
	}

	return &Integration{
		generator: generator,
		enabled:   enabled,
	}
}

// IsEnabled returns whether AI functionality is available
func (ai *Integration) IsEnabled() bool {
	return ai.enabled && ai.generator != nil
}

// GenerateSynopsis creates an AI-powered workspace synopsis
func (ai *Integration) GenerateSynopsis(ctx context.Context, results map[string]*gitexec.RepoStatus) (string, error) {
	if !ai.IsEnabled() {
		return "", ErrAINotEnabled
	}

	synopsisInput := ai.buildSynopsisInput(results)
	return ai.generator.Synopsis(ctx, synopsisInput)
}

// GenerateCommitMessage creates an AI-powered commit message
func (ai *Integration) GenerateCommitMessage(ctx context.Context, input CommitMsgInput) (string, error) {
	if !ai.IsEnabled() {
		return "", ErrAINotEnabled
	}

	return ai.generator.CommitMessage(ctx, input)
}

// GeneratePRReview creates an AI-powered pull request review
func (ai *Integration) GeneratePRReview(ctx context.Context, input PRReviewInput) (string, error) {
	if !ai.IsEnabled() {
		return "", ErrAINotEnabled
	}

	return ai.generator.PRReview(ctx, input)
}

// buildSynopsisInput consolidates synopsis input building logic
func (ai *Integration) buildSynopsisInput(results map[string]*gitexec.RepoStatus) SynopsisInput {
	var repositories []RepoSummary
	totalFiles := 0
	totalLines := 0
	totalCommits := 0

	for repoName, status := range results {
		if status.Error != "" {
			continue // Skip errored repos
		}

		repoSummary := RepoSummary{
			Name:         repoName,
			Branch:       status.Branch,
			Status:       getStatusString(status),
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

	return SynopsisInput{
		Repositories: repositories,
		TotalFiles:   totalFiles,
		TotalLines:   totalLines,
		TotalCommits: totalCommits,
	}
}

// getStatusString provides unified status string representation
func getStatusString(status *gitexec.RepoStatus) string {
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

// Common AI errors
var (
	ErrAINotEnabled = fmt.Errorf("AI functionality not enabled or configured")
)