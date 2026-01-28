package gitexec

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type RepoStatus struct {
	Path        string
	Branch      string
	Dirty       int
	Untracked   int
	Ahead       int
	Behind      int
	HasOrigin   bool
	InProgress  bool
	Error       string

	// 🔥 CYBERPUNK STATS 🔥
	LinesAdded   int
	LinesRemoved int
	FilesChanged int
	Commits      int
	LastCommit   string
	RepoSize     string
}

var junkRegex = regexp.MustCompile(`(?i)(node_modules|\.venv|venv|dist|build|\.tox|\.ruff_cache|\.mypy_cache|\.pytest_cache|\.DS_Store|coverage|\.cache)`)

// DryRunKey is the context key for dry-run mode
type contextKey string
const DryRunKey contextKey = "dry-run"

// IsDryRun checks if the context indicates dry-run mode
func IsDryRun(ctx context.Context) bool {
	if val := ctx.Value(DryRunKey); val != nil {
		if dryRun, ok := val.(bool); ok {
			return dryRun
		}
	}
	return false
}

func Status(ctx context.Context, repoPath string) (*RepoStatus, error) {
	status := &RepoStatus{Path: repoPath}

	branch, err := getCurrentBranch(ctx, repoPath)
	if err != nil {
		status.Error = fmt.Sprintf("get branch: %v", err)
		return status, nil
	}
	status.Branch = branch

	hasOrigin, err := hasOrigin(ctx, repoPath)
	if err != nil {
		status.Error = fmt.Sprintf("check origin: %v", err)
		return status, nil
	}
	status.HasOrigin = hasOrigin

	inProgress, err := isInProgress(ctx, repoPath)
	if err != nil {
		status.Error = fmt.Sprintf("check in-progress: %v", err)
		return status, nil
	}
	status.InProgress = inProgress

	if !hasOrigin || inProgress {
		return status, nil
	}

	dirty, err := getDirtyCount(ctx, repoPath)
	if err != nil {
		status.Error = fmt.Sprintf("get dirty count: %v", err)
		return status, nil
	}
	status.Dirty = dirty

	untracked, err := getUntrackedFiles(ctx, repoPath)
	if err != nil {
		status.Error = fmt.Sprintf("get untracked: %v", err)
		return status, nil
	}
	status.Untracked = len(untracked)

	ahead, behind, err := getAheadBehind(ctx, repoPath)
	if err != nil {
		status.Error = fmt.Sprintf("get ahead/behind: %v", err)
		return status, nil
	}
	status.Ahead = ahead
	status.Behind = behind

	// 🔥 COLLECT CYBERPUNK STATS 🔥
	linesAdded, linesRemoved, filesChanged, err := getDiffStats(ctx, repoPath)
	if err == nil {
		status.LinesAdded = linesAdded
		status.LinesRemoved = linesRemoved
		status.FilesChanged = filesChanged
	}

	commits, err := getCommitCount(ctx, repoPath)
	if err == nil {
		status.Commits = commits
	}

	lastCommit, err := getLastCommitSubject(ctx, repoPath)
	if err == nil {
		status.LastCommit = lastCommit
	}

	repoSize, err := getRepoSize(ctx, repoPath)
	if err == nil {
		status.RepoSize = repoSize
	}

	return status, nil
}

func Preconditions(ctx context.Context, repoPath string) (bool, string) {
	hasOrigin, err := hasOrigin(ctx, repoPath)
	if err != nil {
		return false, fmt.Sprintf("check origin: %v", err)
	}
	if !hasOrigin {
		return false, "no origin remote"
	}

	inProgress, err := isInProgress(ctx, repoPath)
	if err != nil {
		return false, fmt.Sprintf("check progress: %v", err)
	}
	if inProgress {
		return false, "rebase/merge in progress"
	}

	return true, ""
}

func HasJunkFiles(ctx context.Context, repoPath string) (bool, []string, error) {
	untracked, err := getUntrackedFiles(ctx, repoPath)
	if err != nil {
		return false, nil, err
	}

	var junkFiles []string
	for _, file := range untracked {
		if junkRegex.MatchString(file) {
			junkFiles = append(junkFiles, file)
		}
	}

	return len(junkFiles) > 0, junkFiles, nil
}

func Fetch(ctx context.Context, repoPath string) error {
	if IsDryRun(ctx) {
		fmt.Printf("[DRY RUN] Would fetch: git fetch --prune --quiet (in %s)\n", repoPath)
		return nil
	}
	return runGit(ctx, repoPath, "fetch", "--prune", "--quiet")
}

func AddAll(ctx context.Context, repoPath string) error {
	if IsDryRun(ctx) {
		fmt.Printf("[DRY RUN] Would add all: git add -A (in %s)\n", repoPath)
		return nil
	}
	return runGit(ctx, repoPath, "add", "-A")
}

func CommitAllowEmpty(ctx context.Context, repoPath, message string) error {
	if IsDryRun(ctx) {
		fmt.Printf("[DRY RUN] Would commit: git commit --allow-empty -m \"%s\" (in %s)\n", message, repoPath)
		return nil
	}
	return runGit(ctx, repoPath, "commit", "--allow-empty", "-m", message)
}

func SwitchCreate(ctx context.Context, repoPath, branch string) error {
	if IsDryRun(ctx) {
		fmt.Printf("[DRY RUN] Would create/switch branch: git switch -C %s (in %s)\n", branch, repoPath)
		return nil
	}
	return runGit(ctx, repoPath, "switch", "-C", branch)
}

func Switch(ctx context.Context, repoPath, branch string) error {
	if IsDryRun(ctx) {
		fmt.Printf("[DRY RUN] Would switch branch: git switch %s (in %s)\n", branch, repoPath)
		return nil
	}
	return runGit(ctx, repoPath, "switch", branch)
}

func PushUpstream(ctx context.Context, repoPath, branch string) error {
	if IsDryRun(ctx) {
		fmt.Printf("[DRY RUN] Would push with upstream: git push -u origin %s (in %s)\n", branch, repoPath)
		return nil
	}
	return runGit(ctx, repoPath, "push", "-u", "origin", branch)
}

func Push(ctx context.Context, repoPath, branch string) error {
	if IsDryRun(ctx) {
		fmt.Printf("[DRY RUN] Would push: git push origin %s (in %s)\n", branch, repoPath)
		return nil
	}
	return runGit(ctx, repoPath, "push", "origin", branch)
}

func RemoteHasBranch(ctx context.Context, repoPath, branch string) (bool, error) {
	out, err := runGitOutput(ctx, repoPath, "ls-remote", "--heads", "origin", branch)
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(out) != "", nil
}

func Stash(ctx context.Context, repoPath, message string) error {
	if IsDryRun(ctx) {
		fmt.Printf("[DRY RUN] Would stash: git stash push -u -m \"%s\" (in %s)\n", message, repoPath)
		return nil
	}
	return runGit(ctx, repoPath, "stash", "push", "-u", "-m", message)
}

func StashPop(ctx context.Context, repoPath string) error {
	if IsDryRun(ctx) {
		fmt.Printf("[DRY RUN] Would pop stash: git stash pop (in %s)\n", repoPath)
		return nil
	}
	return runGit(ctx, repoPath, "stash", "pop")
}

func HasConflicts(ctx context.Context, repoPath string) (bool, []string, error) {
	out, err := runGitOutput(ctx, repoPath, "diff", "--name-only", "--diff-filter=U")
	if err != nil {
		return false, nil, err
	}

	conflicted := strings.Fields(strings.TrimSpace(out))
	return len(conflicted) > 0, conflicted, nil
}

func LatestRemoteWIP(ctx context.Context, repoPath string) (string, error) {
	out, err := runGitOutput(ctx, repoPath,
		"for-each-ref",
		"--format=%(committerdate:iso8601) %(refname)",
		"refs/remotes/origin/wip/")
	if err != nil {
		return "", err
	}

	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) == 0 || lines[0] == "" {
		return "", fmt.Errorf("no WIP branches found")
	}

	var latest string
	var latestTime time.Time

	for _, line := range lines {
		parts := strings.SplitN(line, " ", 2)
		if len(parts) != 2 {
			continue
		}

		t, err := time.Parse("2006-01-02 15:04:05 -0700", parts[0])
		if err != nil {
			continue
		}

		if latest == "" || t.After(latestTime) {
			latest = parts[1]
			latestTime = t
		}
	}

	if latest == "" {
		return "", fmt.Errorf("no valid WIP branches found")
	}

	return latest, nil
}

func TrimOrigin(remoteRef string) string {
	return strings.TrimPrefix(remoteRef, "refs/remotes/origin/")
}

func DiffNameStatusCached(ctx context.Context, repoPath string) (string, error) {
	return runGitOutput(ctx, repoPath, "diff", "--cached", "--name-status")
}

func DiffStatCached(ctx context.Context, repoPath string) (string, error) {
	return runGitOutput(ctx, repoPath, "diff", "--cached", "--stat")
}

func LogNSubjects(ctx context.Context, repoPath string, n int) ([]string, error) {
	out, err := runGitOutput(ctx, repoPath, "log", fmt.Sprintf("-n%d", n), "--pretty=format:%s")
	if err != nil {
		return nil, err
	}

	lines := strings.Split(strings.TrimSpace(out), "\n")
	var subjects []string
	for _, line := range lines {
		if line != "" {
			subjects = append(subjects, line)
		}
	}
	return subjects, nil
}

func ListUntracked(ctx context.Context, repoPath string) ([]string, error) {
	return getUntrackedFiles(ctx, repoPath)
}

func getCurrentBranch(ctx context.Context, repoPath string) (string, error) {
	return runGitOutput(ctx, repoPath, "rev-parse", "--abbrev-ref", "HEAD")
}

func hasOrigin(ctx context.Context, repoPath string) (bool, error) {
	err := runGit(ctx, repoPath, "remote", "get-url", "origin")
	return err == nil, nil
}

func isInProgress(ctx context.Context, repoPath string) (bool, error) {
	cmd := exec.CommandContext(ctx, "sh", "-c",
		fmt.Sprintf("cd %q && (test -d .git/rebase-apply || test -d .git/rebase-merge || test -f .git/MERGE_HEAD)", repoPath))
	return cmd.Run() == nil, nil
}

func getDirtyCount(ctx context.Context, repoPath string) (int, error) {
	out, err := runGitOutput(ctx, repoPath, "status", "--porcelain")
	if err != nil {
		return 0, err
	}
	return len(strings.Split(strings.TrimSpace(out), "\n")), nil
}

func getUntrackedFiles(ctx context.Context, repoPath string) ([]string, error) {
	out, err := runGitOutput(ctx, repoPath, "ls-files", "--others", "--exclude-standard")
	if err != nil {
		return nil, err
	}

	if strings.TrimSpace(out) == "" {
		return nil, nil
	}

	return strings.Split(strings.TrimSpace(out), "\n"), nil
}

func getAheadBehind(ctx context.Context, repoPath string) (int, int, error) {
	out, err := runGitOutput(ctx, repoPath, "rev-list", "--left-right", "--count", "@{u}...HEAD")
	if err != nil {
		return 0, 0, nil
	}

	parts := strings.Fields(strings.TrimSpace(out))
	if len(parts) != 2 {
		return 0, 0, nil
	}

	behind, _ := strconv.Atoi(parts[0])
	ahead, _ := strconv.Atoi(parts[1])

	return ahead, behind, nil
}

func runGit(ctx context.Context, repoPath string, args ...string) error {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = repoPath
	return cmd.Run()
}

func runGitOutput(ctx context.Context, repoPath string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = repoPath
	out, err := cmd.Output()
	return strings.TrimSpace(string(out)), err
}

// 🔥 CYBERPUNK STAT FUNCTIONS 🔥

// getDiffStats gets lines added/removed and files changed for uncommitted changes
func getDiffStats(ctx context.Context, repoPath string) (linesAdded, linesRemoved, filesChanged int, err error) {
	// Get diff stats for staged and unstaged changes
	out, err := runGitOutput(ctx, repoPath, "diff", "--numstat", "HEAD")
	if err != nil {
		return 0, 0, 0, err
	}

	if strings.TrimSpace(out) == "" {
		return 0, 0, 0, nil
	}

	lines := strings.Split(strings.TrimSpace(out), "\n")
	filesChanged = len(lines)

	for _, line := range lines {
		parts := strings.Fields(line)
		if len(parts) >= 2 {
			if parts[0] != "-" {
				if added, parseErr := strconv.Atoi(parts[0]); parseErr == nil {
					linesAdded += added
				}
			}
			if parts[1] != "-" {
				if removed, parseErr := strconv.Atoi(parts[1]); parseErr == nil {
					linesRemoved += removed
				}
			}
		}
	}

	return linesAdded, linesRemoved, filesChanged, nil
}

// getCommitCount gets total commits in the current branch
func getCommitCount(ctx context.Context, repoPath string) (int, error) {
	out, err := runGitOutput(ctx, repoPath, "rev-list", "--count", "HEAD")
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(out)
}

// getLastCommitSubject gets the subject of the most recent commit
func getLastCommitSubject(ctx context.Context, repoPath string) (string, error) {
	out, err := runGitOutput(ctx, repoPath, "log", "-1", "--pretty=format:%s")
	if err != nil {
		return "", err
	}
	// Truncate if too long
	if len(out) > 50 {
		return out[:47] + "...", nil
	}
	return out, nil
}

// getRepoSize gets approximate repository size
func getRepoSize(ctx context.Context, repoPath string) (string, error) {
	cmd := exec.CommandContext(ctx, "sh", "-c",
		fmt.Sprintf("cd %q && du -sh .git 2>/dev/null | cut -f1", repoPath))
	out, err := cmd.Output()
	if err != nil {
		return "?", nil // Don't fail for size calculation
	}
	size := strings.TrimSpace(string(out))
	if size == "" {
		return "?", nil
	}
	return size, nil
}

func GetLastCommitHash(ctx context.Context, repoPath string) (string, error) {
	output, err := runGitOutput(ctx, repoPath, "rev-parse", "HEAD")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(output), nil
}