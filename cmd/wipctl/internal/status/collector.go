package status

import (
	"context"
	"log/slog"
	"sync"

	"github.com/TheBranchDriftCatalyst/cli-tools/cmd/wipctl/internal/gitexec"
	"github.com/TheBranchDriftCatalyst/cli-tools/cmd/wipctl/internal/workspace"
)

// Collector provides unified status collection across repositories
type Collector struct {
	concurrency int
}

// NewCollector creates a new status collector
func NewCollector(concurrency int) *Collector {
	return &Collector{
		concurrency: concurrency,
	}
}

// CollectStatus gathers status information from all repositories concurrently
func (c *Collector) CollectStatus(ctx context.Context, repos []workspace.Repo) (map[string]*gitexec.RepoStatus, error) {
	results := make(map[string]*gitexec.RepoStatus)
	var mu sync.Mutex
	var wg sync.WaitGroup

	semaphore := make(chan struct{}, c.concurrency)

	for _, repo := range repos {
		wg.Add(1)
		go func(repo workspace.Repo) {
			defer wg.Done()

			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			status, err := gitexec.Status(ctx, repo.Path)
			if err != nil {
				slog.Error("Failed to get repository status",
					"repo", repo.Path,
					"error", err)
				status = &gitexec.RepoStatus{
					Path:  repo.Path,
					Error: err.Error(),
				}
			}

			mu.Lock()
			results[repo.Name] = status
			mu.Unlock()
		}(repo)
	}

	wg.Wait()
	return results, nil
}

// GetStatusString removed - use the version in ai/integration.go instead