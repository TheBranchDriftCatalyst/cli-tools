package workspace

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type Repo struct {
	Path string
	Name string
}

func Discover(ctx context.Context, workspacePath string) ([]Repo, error) {
	var repos []Repo
	var mu sync.Mutex
	var wg sync.WaitGroup

	err := filepath.Walk(workspacePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() {
			return nil
		}

		if info.Name() == ".git" {
			repoPath := filepath.Dir(path)
			repoName := getRepoName(repoPath)

			wg.Add(1)
			go func() {
				defer wg.Done()

				if isValidGitRepo(repoPath) {
					repo := Repo{
						Path: repoPath,
						Name: repoName,
					}

					mu.Lock()
					repos = append(repos, repo)
					mu.Unlock()
				}
			}()

			return filepath.SkipDir
		}

		return nil
	})

	wg.Wait()

	if err != nil {
		return nil, fmt.Errorf("workspace discovery failed: %w", err)
	}

	return repos, nil
}

func isValidGitRepo(path string) bool {
	gitDir := filepath.Join(path, ".git")

	info, err := os.Stat(gitDir)
	if err != nil {
		return false
	}

	if info.IsDir() {
		return true
	}

	if !info.IsDir() {
		content, err := os.ReadFile(gitDir)
		if err != nil {
			return false
		}
		return len(content) > 0 && string(content[:8]) == "gitdir: "
	}

	return false
}

// getRepoName returns a meaningful repository name, handling edge cases like current directory
func getRepoName(repoPath string) string {
	name := filepath.Base(repoPath)

	// Handle current directory case
	if name == "." {
		// Try to get the actual directory name
		absPath, err := filepath.Abs(repoPath)
		if err == nil {
			name = filepath.Base(absPath)
		}
	}

	// Handle root directory or other edge cases
	if name == "" || name == "/" || name == "\\" {
		name = "root"
	}

	// Clean up the name
	name = strings.TrimSpace(name)
	if name == "" {
		name = "unknown"
	}

	return name
}