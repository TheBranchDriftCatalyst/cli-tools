package processor

import (
	"context"
	"fmt"
	"sync"

	"github.com/TheBranchDriftCatalyst/cli-tools/cmd/wipctl/internal/report"
	"github.com/TheBranchDriftCatalyst/cli-tools/cmd/wipctl/internal/ui"
	"github.com/TheBranchDriftCatalyst/cli-tools/cmd/wipctl/internal/workspace"
)

// ProcessorConfig unified configuration for all workspace operations
type ProcessorConfig struct {
	Concurrency   int
	Operation     string
	WorkspacePath string
	ReportDir     string
	Verbose       bool
}

// RepoHandler defines the interface for repository-specific operations
type RepoHandler interface {
	ProcessRepo(ctx context.Context, repo workspace.Repo) report.ReportEntry
	RequiresPreconditions() bool
	RequiresReport() bool
	GetOperationName() string
}

// WorkspaceProcessor unified processor for all repository operations
type WorkspaceProcessor struct {
	config ProcessorConfig
	report *report.Report
}

// NewProcessor creates a new unified workspace processor
func NewProcessor(config ProcessorConfig) *WorkspaceProcessor {
	return &WorkspaceProcessor{
		config: config,
	}
}

// ProcessWorkspace executes the handler across all repositories in the workspace
func (p *WorkspaceProcessor) ProcessWorkspace(ctx context.Context, handler RepoHandler) error {
	// Discover repositories
	ui.Info("Discovering Git repositories...")
	repos, err := workspace.Discover(ctx, p.config.WorkspacePath)
	if err != nil {
		ui.Error("Failed to discover repositories: " + err.Error())
		return err
	}

	if len(repos) == 0 {
		ui.Warning("No Git repositories found in workspace")
		return nil
	}

	ui.Info(fmt.Sprintf("Processing %d repositories for %s operation...", len(repos), handler.GetOperationName()))

	// Initialize report if needed
	if handler.RequiresReport() {
		p.report = report.NewReport(
			fmt.Sprintf("%s Report", handler.GetOperationName()),
			p.config.WorkspacePath,
			p.config.ReportDir,
			p.config.Operation,
		)
	}

	// Process repositories concurrently
	return p.processConcurrently(ctx, repos, handler)
}

// processConcurrently handles the unified concurrency pattern
func (p *WorkspaceProcessor) processConcurrently(ctx context.Context, repos []workspace.Repo, handler RepoHandler) error {
	var mu sync.Mutex
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, p.config.Concurrency)

	for _, repo := range repos {
		wg.Add(1)
		go func(repo workspace.Repo) {
			defer wg.Done()

			// Unified semaphore pattern
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			// Process repository using handler
			entry := handler.ProcessRepo(ctx, repo)

			// Thread-safe result collection
			if handler.RequiresReport() && p.report != nil {
				mu.Lock()
				p.report.AddEntry(entry)
				mu.Unlock()
			}
		}(repo)
	}

	wg.Wait()

	// Save report if required
	if handler.RequiresReport() && p.report != nil {
		if err := p.report.Save(); err != nil {
			ui.Warning("Failed to save report: " + err.Error())
		} else {
			ui.Success(fmt.Sprintf("%s operation completed. Report saved.", handler.GetOperationName()))
		}
	}

	return nil
}

// GetReport returns the generated report (if any)
func (p *WorkspaceProcessor) GetReport() *report.Report {
	return p.report
}