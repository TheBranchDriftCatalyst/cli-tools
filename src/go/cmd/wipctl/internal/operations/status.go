package operations

import (
	"context"
	"fmt"

	"github.com/TheBranchDriftCatalyst/dotfiles-2024/cli/wipctl/internal/ai"
	"github.com/TheBranchDriftCatalyst/dotfiles-2024/cli/wipctl/internal/gitexec"
	"github.com/TheBranchDriftCatalyst/dotfiles-2024/cli/wipctl/internal/report"
	"github.com/TheBranchDriftCatalyst/dotfiles-2024/cli/wipctl/internal/status"
	"github.com/TheBranchDriftCatalyst/dotfiles-2024/cli/wipctl/internal/ui"
	"github.com/TheBranchDriftCatalyst/dotfiles-2024/cli/wipctl/internal/workspace"
)

// StatusHandler handles repository status display operations
type StatusHandler struct {
	withAI      bool
	aiIntegration *ai.Integration
	collector   *status.Collector
}

// NewStatusHandler creates a new status operation handler
func NewStatusHandler(withAI bool, concurrency int) *StatusHandler {
	var aiIntegration *ai.Integration
	if withAI {
		aiIntegration = ai.NewIntegration()
	}

	return &StatusHandler{
		withAI:        withAI,
		aiIntegration: aiIntegration,
		collector:     status.NewCollector(concurrency),
	}
}

// ProcessRepo implements RepoHandler interface for status display
func (h *StatusHandler) ProcessRepo(ctx context.Context, repo workspace.Repo) report.ReportEntry {
	// Status operation doesn't use individual repo processing
	// All status collection is handled in bulk by ProcessWorkspaceStatus
	return report.ReportEntry{}
}

// ProcessWorkspaceStatus handles the complete status operation workflow
func (h *StatusHandler) ProcessWorkspaceStatus(ctx context.Context, repos []workspace.Repo) error {
	// Collect status from all repositories
	results, err := h.collector.CollectStatus(ctx, repos)
	if err != nil {
		return err
	}

	// Display the status table
	h.displayStatusTable(results)

	// Generate AI synopsis if requested
	if h.withAI && h.aiIntegration != nil && h.aiIntegration.IsEnabled() {
		h.displayAISynopsis(ctx, results)
	}

	return nil
}

// displayStatusTable renders the repository status table
func (h *StatusHandler) displayStatusTable(results map[string]*gitexec.RepoStatus) {
	ui.InitTable("Repository", "Branch", "Status", "Files", "Lines", "Commits", "Ahead", "Behind", "Size")

	for repoName, status := range results {
		if status.Error != "" {
			ui.AddTableRow(
				ui.CyberText(repoName, "repo"),
				ui.StatusCell("error"),
				"-", "-", "-", "-", "-", "-", "-",
			)
			ui.Error(repoName + ": " + status.Error)
			continue
		}

		if !status.HasOrigin {
			ui.AddTableRow(
				ui.CyberText(repoName, "repo"),
				ui.CyberText(status.Branch, "branch"),
				ui.StatusCell("no-origin"),
				"-", "-", "-", "-", "-", "-",
			)
			continue
		}

		if status.InProgress {
			ui.AddTableRow(
				ui.CyberText(repoName, "repo"),
				ui.CyberText(status.Branch, "branch"),
				ui.StatusCell("in-progress"),
				"-", "-", "-", "-", "-", "-",
			)
			continue
		}

		// Normal status display
		statusStr := ui.StatusCell("clean")
		if status.Dirty > 0 {
			statusStr = ui.StatusCell("dirty")
		}

		ui.AddTableRow(
			ui.CyberText(repoName, "repo"),
			ui.CyberText(status.Branch, "branch"),
			statusStr,
			ui.SynthwaveNumber(status.FilesChanged, "files"),
			h.formatLineChanges(status),
			ui.SynthwaveNumber(status.Commits, "commits"),
			ui.SynthwaveNumber(status.Ahead, "ahead"),
			ui.SynthwaveNumber(status.Behind, "behind"),
			ui.CyberText(status.RepoSize, "size"),
		)
	}

	ui.RenderTable()
	ui.Info("System operational - All repositories scanned")
}

// displayAISynopsis generates and displays AI-powered synopsis
func (h *StatusHandler) displayAISynopsis(ctx context.Context, results map[string]*gitexec.RepoStatus) {
	ui.Info("🤖 Generating AI workspace synopsis...")

	synopsis, err := h.aiIntegration.GenerateSynopsis(ctx, results)
	if err != nil {
		ui.Warning("AI synopsis failed: " + err.Error())
		return
	}

	ui.Success("🧠 AI Workspace Intelligence")
	ui.Info("─────────────────────────────")
	println(synopsis)
}

// formatLineChanges formats line addition/removal display
func (h *StatusHandler) formatLineChanges(status *gitexec.RepoStatus) string {
	if status.LinesAdded > 0 || status.LinesRemoved > 0 {
		return fmt.Sprintf("+%d/-%d", status.LinesAdded, status.LinesRemoved)
	}
	return "—"
}

// RequiresPreconditions returns false as status checking doesn't need preconditions
func (h *StatusHandler) RequiresPreconditions() bool {
	return false
}

// RequiresReport returns false as status doesn't generate reports
func (h *StatusHandler) RequiresReport() bool {
	return false
}

// GetOperationName returns the operation name for display
func (h *StatusHandler) GetOperationName() string {
	return "Status"
}