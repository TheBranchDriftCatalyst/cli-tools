package report

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type CheckpointEntry struct {
	ReportEntry
	// Enhanced checkpoint-specific fields
	Branch          string   `json:"branch"`
	FilesModified   int      `json:"files_modified"`
	FilesAdded      int      `json:"files_added"`
	LinesAdded      int      `json:"lines_added"`
	LinesRemoved    int      `json:"lines_removed"`
	CommitHash      string   `json:"commit_hash"`
	WipBranch       string   `json:"wip_branch"`
	RecentCommits   []string `json:"recent_commits"`
	ChangedFiles    []string `json:"changed_files"`
	CommitMessage   string   `json:"commit_message"`
	FeatureName     string   `json:"feature_name,omitempty"`
	CrossRepoGroup  string   `json:"cross_repo_group,omitempty"`
}

type CheckpointReport struct {
	*Report
	// Workspace-wide analysis
	TotalRepos       int                `json:"total_repos"`
	ProcessedRepos   int                `json:"processed_repos"`
	SuccessfulRepos  int                `json:"successful_repos"`
	FailedRepos      int                `json:"failed_repos"`
	SkippedRepos     int                `json:"skipped_repos"`
	TotalFiles       int                `json:"total_files"`
	TotalLines       int                `json:"total_lines"`
	WorkspaceChanges string             `json:"workspace_changes"`
	FeatureName      string             `json:"feature_name,omitempty"`
	CrossRepoEnabled bool               `json:"cross_repo_enabled"`
	Entries          []CheckpointEntry  `json:"entries"`
}

func NewCheckpointReport(title, workspace, reportDir string, featureName string, crossRepo bool) *CheckpointReport {
	baseReport := NewReport(title, workspace, reportDir, "checkpoint")
	return &CheckpointReport{
		Report:           baseReport,
		FeatureName:      featureName,
		CrossRepoEnabled: crossRepo,
		Entries:          []CheckpointEntry{},
	}
}

func (r *CheckpointReport) AddCheckpointEntry(entry CheckpointEntry) {
	r.Entries = append(r.Entries, entry)

	// Update totals
	r.ProcessedRepos++
	switch entry.Outcome {
	case "success":
		r.SuccessfulRepos++
		r.TotalFiles += entry.FilesModified + entry.FilesAdded
		r.TotalLines += entry.LinesAdded + entry.LinesRemoved
	case "failed":
		r.FailedRepos++
	case "skipped":
		r.SkippedRepos++
	}
}

func (r *CheckpointReport) GenerateWorkspaceSummary() {
	var summaryParts []string

	if r.SuccessfulRepos > 0 {
		summaryParts = append(summaryParts, fmt.Sprintf("%d repos checkpointed", r.SuccessfulRepos))
	}
	if r.TotalFiles > 0 {
		summaryParts = append(summaryParts, fmt.Sprintf("%d files changed", r.TotalFiles))
	}
	if r.TotalLines > 0 {
		summaryParts = append(summaryParts, fmt.Sprintf("%d lines modified", r.TotalLines))
	}
	if r.FailedRepos > 0 {
		summaryParts = append(summaryParts, fmt.Sprintf("%d failures", r.FailedRepos))
	}

	r.WorkspaceChanges = strings.Join(summaryParts, ", ")
}

func (r *CheckpointReport) Save() error {
	if err := os.MkdirAll(r.reportDir, 0755); err != nil {
		return fmt.Errorf("create report directory: %w", err)
	}

	filename := fmt.Sprintf("wip-%s-%s.md", r.operation, r.Timestamp.Format("20060102-150405"))
	filepath := filepath.Join(r.reportDir, filename)

	content := r.generateEnhancedMarkdown()

	if err := os.WriteFile(filepath, []byte(content), 0644); err != nil {
		return fmt.Errorf("write report file: %w", err)
	}

	return nil
}

func (r *CheckpointReport) generateEnhancedMarkdown() string {
	var sb strings.Builder

	// Header with enhanced metadata
	sb.WriteString(fmt.Sprintf("# %s\n\n", r.Title))
	sb.WriteString(fmt.Sprintf("**Workspace:** %s  \n", r.Workspace))
	sb.WriteString(fmt.Sprintf("**Timestamp:** %s  \n", r.Timestamp.Format(time.RFC3339)))
	sb.WriteString(fmt.Sprintf("**Total Repositories:** %d  \n", r.TotalRepos))

	if r.FeatureName != "" {
		sb.WriteString(fmt.Sprintf("**Feature:** %s  \n", r.FeatureName))
	}
	if r.CrossRepoEnabled {
		sb.WriteString("**Cross-Repo Mode:** Enabled  \n")
	}
	sb.WriteString("\n")

	// Workspace Summary
	sb.WriteString("## 📊 Workspace Summary\n\n")
	if r.WorkspaceChanges != "" {
		sb.WriteString(fmt.Sprintf("**Changes:** %s  \n", r.WorkspaceChanges))
	}
	sb.WriteString(fmt.Sprintf("- ✅ **Successful:** %d repositories\n", r.SuccessfulRepos))
	if r.FailedRepos > 0 {
		sb.WriteString(fmt.Sprintf("- ❌ **Failed:** %d repositories\n", r.FailedRepos))
	}
	if r.SkippedRepos > 0 {
		sb.WriteString(fmt.Sprintf("- ⏭️ **Skipped:** %d repositories\n", r.SkippedRepos))
	}
	sb.WriteString(fmt.Sprintf("- 📁 **Total Files:** %d\n", r.TotalFiles))
	sb.WriteString(fmt.Sprintf("- 📝 **Total Lines:** %d\n", r.TotalLines))
	sb.WriteString("\n")

	// Cross-repo feature analysis
	if r.CrossRepoEnabled && r.FeatureName != "" {
		sb.WriteString("## 🔗 Cross-Repository Feature Analysis\n\n")
		sb.WriteString(fmt.Sprintf("**Feature Name:** `%s`\n\n", r.FeatureName))

		featureRepos := []string{}
		for _, entry := range r.Entries {
			if entry.FeatureName == r.FeatureName && entry.Outcome == "success" {
				featureRepos = append(featureRepos, entry.Repo)
			}
		}

		if len(featureRepos) > 0 {
			sb.WriteString("**Repositories in Feature:**\n")
			for _, repo := range featureRepos {
				sb.WriteString(fmt.Sprintf("- %s\n", repo))
			}
			sb.WriteString("\n")
		}
	}

	// Detailed repository results
	sb.WriteString("## 📋 Repository Details\n\n")

	if len(r.Entries) == 0 {
		sb.WriteString("No repositories processed.\n")
		return sb.String()
	}

	for _, entry := range r.Entries {
		sb.WriteString(r.formatCheckpointEntry(entry))
		sb.WriteString("\n")
	}

	return sb.String()
}

func (r *CheckpointReport) formatCheckpointEntry(entry CheckpointEntry) string {
	var sb strings.Builder

	// Repository header with status
	statusIcon := "✅"
	if entry.Outcome == "failed" {
		statusIcon = "❌"
	} else if entry.Outcome == "skipped" {
		statusIcon = "⏭️"
	}

	sb.WriteString(fmt.Sprintf("### %s **%s** (%s)\n\n", statusIcon, entry.Repo, entry.Branch))

	// Basic info
	sb.WriteString(fmt.Sprintf("- **Status:** %s\n", entry.Outcome))
	if entry.Details != "" {
		sb.WriteString(fmt.Sprintf("- **Details:** %s\n", entry.Details))
	}

	// Success-specific details
	if entry.Outcome == "success" {
		if entry.WipBranch != "" {
			sb.WriteString(fmt.Sprintf("- **WIP Branch:** `%s`\n", entry.WipBranch))
		}
		if entry.CommitHash != "" {
			sb.WriteString(fmt.Sprintf("- **Commit:** `%s`\n", entry.CommitHash))
		}
		if entry.CommitMessage != "" {
			sb.WriteString(fmt.Sprintf("- **Message:** %s\n", entry.CommitMessage))
		}

		// File statistics
		if entry.FilesModified > 0 || entry.FilesAdded > 0 {
			sb.WriteString(fmt.Sprintf("- **Files:** %d modified, %d added\n",
				entry.FilesModified, entry.FilesAdded))
		}
		if entry.LinesAdded > 0 || entry.LinesRemoved > 0 {
			sb.WriteString(fmt.Sprintf("- **Lines:** +%d/-%d\n",
				entry.LinesAdded, entry.LinesRemoved))
		}

		// Feature coordination
		if entry.FeatureName != "" {
			sb.WriteString(fmt.Sprintf("- **Feature:** `%s`\n", entry.FeatureName))
		}

		// Changed files
		if len(entry.ChangedFiles) > 0 {
			sb.WriteString("- **Changed Files:**\n")
			for _, file := range entry.ChangedFiles {
				sb.WriteString(fmt.Sprintf("  - `%s`\n", file))
			}
		}

		// Recent commits context
		if len(entry.RecentCommits) > 0 {
			sb.WriteString("- **Recent Commits:**\n")
			for _, commit := range entry.RecentCommits {
				sb.WriteString(fmt.Sprintf("  - %s\n", commit))
			}
		}
	}

	// Warnings and errors
	for _, warning := range entry.Warnings {
		sb.WriteString(fmt.Sprintf("  ⚠️ %s\n", warning))
	}
	for _, err := range entry.Errors {
		sb.WriteString(fmt.Sprintf("  ❌ %s\n", err))
	}

	return sb.String()
}

func CreateCheckpointEntry(repo, branch, outcome, details string) CheckpointEntry {
	return CheckpointEntry{
		ReportEntry: ReportEntry{
			Repo:    repo,
			Outcome: outcome,
			Details: details,
		},
		Branch: branch,
	}
}