package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
)

type CommitMsgInput struct {
	Repo          string   `json:"repo"`
	Branch        string   `json:"branch"`
	Host          string   `json:"host"`
	NameStatus    string   `json:"name_status"`
	DiffStat      string   `json:"diff_stat"`
	Untracked     []string `json:"untracked"`
	PriorSubjects []string `json:"prior_subjects"`
}

type Generator interface {
	CommitMessage(ctx context.Context, input CommitMsgInput) (string, error)
	Synopsis(ctx context.Context, input SynopsisInput) (string, error)
	PRReview(ctx context.Context, input PRReviewInput) (string, error)
	WorkspaceContext(ctx context.Context, input WorkspaceContextInput) (string, error)
}

type SynopsisInput struct {
	Repositories []RepoSummary `json:"repositories"`
	TotalFiles   int           `json:"total_files"`
	TotalLines   int           `json:"total_lines"`
	TotalCommits int           `json:"total_commits"`
}

type RepoSummary struct {
	Name         string `json:"name"`
	Branch       string `json:"branch"`
	Status       string `json:"status"`
	FilesChanged int    `json:"files_changed"`
	LinesAdded   int    `json:"lines_added"`
	LinesRemoved int    `json:"lines_removed"`
	Commits      int    `json:"commits"`
}

type PRReviewInput struct {
	Repo        string   `json:"repo"`
	Branch      string   `json:"branch"`
	DiffStat    string   `json:"diff_stat"`
	NameStatus  string   `json:"name_status"`
	CommitMsgs  []string `json:"commit_messages"`
	FilesCount  int      `json:"files_count"`
	LinesAdded  int      `json:"lines_added"`
	LinesRemoved int     `json:"lines_removed"`
}

type WorkspaceContextInput struct {
	Repositories []WorkspaceRepo `json:"repositories"`
	TotalFiles   int             `json:"total_files"`
	TotalLines   int             `json:"total_lines"`
	TotalCommits int             `json:"total_commits"`
	ActiveRepos  int             `json:"active_repos"`
	DirtyRepos   int             `json:"dirty_repos"`
}

type WorkspaceRepo struct {
	Name         string   `json:"name"`
	Branch       string   `json:"branch"`
	Status       string   `json:"status"`
	FilesChanged int      `json:"files_changed"`
	LinesAdded   int      `json:"lines_added"`
	LinesRemoved int      `json:"lines_removed"`
	Commits      int      `json:"commits"`
	RecentWork   []string `json:"recent_work"`   // Recent commit messages
	Changes      string   `json:"changes"`       // What files changed
}

type Config struct {
	Provider    string
	Endpoint    string
	Model       string
	Token       string
	ExecPath    string
	MaxTokens   int
	Temperature float64
}

func NewGenerator(config Config) Generator {
	switch config.Provider {
	case "exec":
		return &ExecGenerator{execPath: config.ExecPath}
	case "openai":
		return &OpenAIGenerator{
			endpoint:    config.Endpoint,
			model:       config.Model,
			token:       config.Token,
			maxTokens:   config.MaxTokens,
			temperature: config.Temperature,
		}
	case "claude", "anthropic":
		return &ClaudeGenerator{
			endpoint:    config.Endpoint,
			model:       config.Model,
			token:       config.Token,
			maxTokens:   config.MaxTokens,
			temperature: config.Temperature,
		}
	case "ollama":
		return &OllamaGenerator{
			endpoint: config.Endpoint,
			model:    config.Model,
		}
	default:
		return &NoneGenerator{}
	}
}

type NoneGenerator struct{}

func (g *NoneGenerator) CommitMessage(ctx context.Context, input CommitMsgInput) (string, error) {
	return fmt.Sprintf("chore(wip): checkpoint %s (%s)", input.Host, input.Branch), nil
}

func (g *NoneGenerator) Synopsis(ctx context.Context, input SynopsisInput) (string, error) {
	return "No AI provider configured - workspace synopsis unavailable", nil
}

func (g *NoneGenerator) PRReview(ctx context.Context, input PRReviewInput) (string, error) {
	return "No AI provider configured - PR review unavailable", nil
}

func (g *NoneGenerator) WorkspaceContext(ctx context.Context, input WorkspaceContextInput) (string, error) {
	return "No AI provider configured - workspace context unavailable", nil
}

type ExecGenerator struct {
	execPath string
}

func (g *ExecGenerator) CommitMessage(ctx context.Context, input CommitMsgInput) (string, error) {
	return g.execCommand(ctx, "commit", input)
}

func (g *ExecGenerator) Synopsis(ctx context.Context, input SynopsisInput) (string, error) {
	return g.execCommand(ctx, "synopsis", input)
}

func (g *ExecGenerator) PRReview(ctx context.Context, input PRReviewInput) (string, error) {
	return g.execCommand(ctx, "prreview", input)
}

func (g *ExecGenerator) WorkspaceContext(ctx context.Context, input WorkspaceContextInput) (string, error) {
	return g.execCommand(ctx, "workspace", input)
}

func (g *ExecGenerator) execCommand(ctx context.Context, command string, input interface{}) (string, error) {
	if g.execPath == "" {
		return "", fmt.Errorf("exec path not configured")
	}

	inputJson, err := json.Marshal(map[string]interface{}{
		"command": command,
		"input":   input,
	})
	if err != nil {
		return "", fmt.Errorf("marshal input: %w", err)
	}

	cmd := exec.CommandContext(ctx, g.execPath)
	cmd.Stdin = bytes.NewReader(inputJson)

	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("exec command failed: %w", err)
	}

	return strings.TrimSpace(string(output)), nil
}

type OpenAIGenerator struct {
	endpoint    string
	model       string
	token       string
	maxTokens   int
	temperature float64
}

func (g *OpenAIGenerator) CommitMessage(ctx context.Context, input CommitMsgInput) (string, error) {
	systemPrompt := "You are an expert helping developers write precise Git commit messages. Use conventional commits when possible (feat|fix|chore|refactor|docs|test|build|ci|perf). Keep a one-line subject (<= 72 chars). Add a short body with bullets if needed. No code fences."

	userPrompt := g.buildPrompt(input)

	reqBody := map[string]interface{}{
		"model": g.model,
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": userPrompt},
		},
		"max_tokens":  g.maxTokens,
		"temperature": g.temperature,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", g.endpoint+"/v1/chat/completions", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if g.token != "" {
		req.Header.Set("Authorization", "Bearer "+g.token)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	var response struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}

	if len(response.Choices) == 0 {
		return "", fmt.Errorf("no choices in response")
	}

	return strings.TrimSpace(response.Choices[0].Message.Content), nil
}

func (g *OpenAIGenerator) Synopsis(ctx context.Context, input SynopsisInput) (string, error) {
	systemPrompt := "You are an expert developer creating workspace intelligence reports. Generate a concise, professional synopsis of development activity across repositories."
	userPrompt := g.buildSynopsisPrompt(input)
	return g.makeRequest(ctx, systemPrompt, userPrompt)
}

func (g *OpenAIGenerator) PRReview(ctx context.Context, input PRReviewInput) (string, error) {
	systemPrompt := "You are an expert code reviewer. Provide a thorough but concise PR review with actionable feedback."
	userPrompt := g.buildPRReviewPrompt(input)
	return g.makeRequest(ctx, systemPrompt, userPrompt)
}

func (g *OpenAIGenerator) makeRequest(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	reqBody := map[string]interface{}{
		"model": g.model,
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": userPrompt},
		},
		"max_tokens":  g.maxTokens,
		"temperature": g.temperature,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", g.endpoint+"/v1/chat/completions", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if g.token != "" {
		req.Header.Set("Authorization", "Bearer "+g.token)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	var response struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}

	if len(response.Choices) == 0 {
		return "", fmt.Errorf("no choices in response")
	}

	return strings.TrimSpace(response.Choices[0].Message.Content), nil
}

// 🔥 CLAUDE API GENERATOR - CYBERPUNK INTELLIGENCE 🔥
type ClaudeGenerator struct {
	endpoint    string
	model       string
	token       string
	maxTokens   int
	temperature float64
}

func (g *ClaudeGenerator) CommitMessage(ctx context.Context, input CommitMsgInput) (string, error) {
	systemPrompt := "You are an expert helping developers write precise Git commit messages. Use conventional commits when possible (feat|fix|chore|refactor|docs|test|build|ci|perf). Keep a one-line subject (<= 72 chars). Add a short body with bullets if needed. No code fences."
	userPrompt := g.buildCommitPrompt(input)
	return g.makeClaudeRequest(ctx, systemPrompt, userPrompt)
}

func (g *ClaudeGenerator) Synopsis(ctx context.Context, input SynopsisInput) (string, error) {
	systemPrompt := "You are an expert developer creating workspace intelligence reports. Generate a concise, professional synopsis of development activity across repositories. Focus on key insights and patterns."
	userPrompt := g.buildSynopsisPrompt(input)
	return g.makeClaudeRequest(ctx, systemPrompt, userPrompt)
}

func (g *ClaudeGenerator) PRReview(ctx context.Context, input PRReviewInput) (string, error) {
	systemPrompt := "You are an expert code reviewer. Provide a thorough but concise PR review with actionable feedback. Focus on code quality, potential issues, and improvement suggestions."
	userPrompt := g.buildPRReviewPrompt(input)
	return g.makeClaudeRequest(ctx, systemPrompt, userPrompt)
}

func (g *ClaudeGenerator) WorkspaceContext(ctx context.Context, input WorkspaceContextInput) (string, error) {
	systemPrompt := `You are a development session assistant helping a developer understand where they left off in their work.

Your role is to analyze their workspace state and provide a clear, actionable briefing that answers:
- What was I working on when I stopped?
- Which repositories have active work?
- What's the current state of each project?
- Where should I start when I return to work?
- What are the next logical steps?

Focus on work session continuity, not code quality. This is for "future me" context passing.`

	userPrompt := g.buildWorkspaceContextPrompt(input)
	return g.makeClaudeRequest(ctx, systemPrompt, userPrompt)
}

func (g *ClaudeGenerator) makeClaudeRequest(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	endpoint := g.endpoint
	if endpoint == "" {
		endpoint = "https://api.anthropic.com"
	}

	reqBody := map[string]interface{}{
		"model":      g.model,
		"max_tokens": g.maxTokens,
		"temperature": g.temperature,
		"messages": []map[string]string{
			{"role": "user", "content": systemPrompt + "\n\n" + userPrompt},
		},
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", endpoint+"/v1/messages", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("anthropic-version", "2023-06-01")
	if g.token != "" {
		req.Header.Set("x-api-key", g.token)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	var response struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}

	if len(response.Content) == 0 {
		return "", fmt.Errorf("no content in response")
	}

	return strings.TrimSpace(response.Content[0].Text), nil
}

type OllamaGenerator struct {
	endpoint string
	model    string
}

func (g *OllamaGenerator) CommitMessage(ctx context.Context, input CommitMsgInput) (string, error) {
	endpoint := g.endpoint
	if endpoint == "" {
		endpoint = "http://localhost:11434"
	}

	systemPrompt := "You are an expert helping developers write precise Git commit messages. Use conventional commits when possible (feat|fix|chore|refactor|docs|test|build|ci|perf). Keep a one-line subject (<= 72 chars). Add a short body with bullets if needed. No code fences."

	userPrompt := (&OpenAIGenerator{}).buildPrompt(input)
	fullPrompt := systemPrompt + "\n\n" + userPrompt

	reqBody := map[string]interface{}{
		"model":  g.model,
		"prompt": fullPrompt,
		"stream": false,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", endpoint+"/api/generate", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	var response struct {
		Response string `json:"response"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}

	return strings.TrimSpace(response.Response), nil
}

func (g *OllamaGenerator) Synopsis(ctx context.Context, input SynopsisInput) (string, error) {
	systemPrompt := "You are an expert developer creating workspace intelligence reports. Generate a concise, professional synopsis of development activity across repositories."
	userPrompt := (&ClaudeGenerator{}).buildSynopsisPrompt(input)
	return g.makeOllamaRequest(ctx, systemPrompt, userPrompt)
}

func (g *OllamaGenerator) PRReview(ctx context.Context, input PRReviewInput) (string, error) {
	systemPrompt := "You are an expert code reviewer. Provide a thorough but concise PR review with actionable feedback."
	userPrompt := (&ClaudeGenerator{}).buildPRReviewPrompt(input)
	return g.makeOllamaRequest(ctx, systemPrompt, userPrompt)
}

func (g *OllamaGenerator) WorkspaceContext(ctx context.Context, input WorkspaceContextInput) (string, error) {
	systemPrompt := "You are a development session assistant helping a developer understand where they left off in their work. Focus on work session continuity, not code quality. This is for 'future me' context passing."
	userPrompt := (&ClaudeGenerator{}).buildWorkspaceContextPrompt(input)
	return g.makeOllamaRequest(ctx, systemPrompt, userPrompt)
}

func (g *OllamaGenerator) makeOllamaRequest(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	endpoint := g.endpoint
	if endpoint == "" {
		endpoint = "http://localhost:11434"
	}

	fullPrompt := systemPrompt + "\n\n" + userPrompt

	reqBody := map[string]interface{}{
		"model":  g.model,
		"prompt": fullPrompt,
		"stream": false,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", endpoint+"/api/generate", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	var response struct {
		Response string `json:"response"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}

	return strings.TrimSpace(response.Response), nil
}

func (g *OpenAIGenerator) buildPrompt(input CommitMsgInput) string {
	var parts []string

	parts = append(parts, fmt.Sprintf("Repository: %s", input.Repo))
	parts = append(parts, fmt.Sprintf("Branch: %s", input.Branch))
	parts = append(parts, fmt.Sprintf("Host: %s", input.Host))

	if input.NameStatus != "" {
		parts = append(parts, "\nFile Changes:")
		parts = append(parts, input.NameStatus)
	}

	if input.DiffStat != "" {
		parts = append(parts, "\nDiff Summary:")
		parts = append(parts, input.DiffStat)
	}

	if len(input.Untracked) > 0 {
		parts = append(parts, fmt.Sprintf("\nUntracked files: %s", strings.Join(input.Untracked, ", ")))
	}

	if len(input.PriorSubjects) > 0 {
		parts = append(parts, "\nRecent commit messages:")
		for _, subject := range input.PriorSubjects {
			parts = append(parts, "- "+subject)
		}
	}

	parts = append(parts, "\nGenerate a concise commit message for these changes:")

	return strings.Join(parts, "\n")
}

// 🔥 CLAUDE PROMPT BUILDERS 🔥

func (g *ClaudeGenerator) buildCommitPrompt(input CommitMsgInput) string {
	return (&OpenAIGenerator{}).buildPrompt(input) // Reuse existing commit prompt logic
}

func (g *ClaudeGenerator) buildSynopsisPrompt(input SynopsisInput) string {
	var parts []string

	parts = append(parts, "WORKSPACE INTELLIGENCE SYNOPSIS")
	parts = append(parts, "=====================================")
	parts = append(parts, fmt.Sprintf("Total Repositories: %d", len(input.Repositories)))
	parts = append(parts, fmt.Sprintf("Total Files Changed: %d", input.TotalFiles))
	parts = append(parts, fmt.Sprintf("Total Lines Changed: %d", input.TotalLines))
	parts = append(parts, fmt.Sprintf("Total Commits: %d", input.TotalCommits))
	parts = append(parts, "")

	parts = append(parts, "REPOSITORY DETAILS:")
	for _, repo := range input.Repositories {
		parts = append(parts, fmt.Sprintf("• %s (%s):", repo.Name, repo.Branch))
		parts = append(parts, fmt.Sprintf("  Status: %s", repo.Status))
		if repo.FilesChanged > 0 {
			parts = append(parts, fmt.Sprintf("  Files: %d, Lines: +%d/-%d, Commits: %d",
				repo.FilesChanged, repo.LinesAdded, repo.LinesRemoved, repo.Commits))
		}
		parts = append(parts, "")
	}

	parts = append(parts, "Please generate a concise executive summary of this workspace activity.")
	parts = append(parts, "Focus on:")
	parts = append(parts, "- Overall development patterns")
	parts = append(parts, "- Key areas of activity")
	parts = append(parts, "- Notable insights or trends")
	parts = append(parts, "- Brief assessment of workspace health")

	return strings.Join(parts, "\n")
}

func (g *ClaudeGenerator) buildPRReviewPrompt(input PRReviewInput) string {
	var parts []string

	parts = append(parts, "PULL REQUEST REVIEW REQUEST")
	parts = append(parts, "===========================")
	parts = append(parts, fmt.Sprintf("Repository: %s", input.Repo))
	parts = append(parts, fmt.Sprintf("Branch: %s", input.Branch))
	parts = append(parts, fmt.Sprintf("Files Changed: %d", input.FilesCount))
	parts = append(parts, fmt.Sprintf("Lines: +%d/-%d", input.LinesAdded, input.LinesRemoved))
	parts = append(parts, "")

	if input.NameStatus != "" {
		parts = append(parts, "FILE CHANGES:")
		parts = append(parts, input.NameStatus)
		parts = append(parts, "")
	}

	if input.DiffStat != "" {
		parts = append(parts, "DIFF SUMMARY:")
		parts = append(parts, input.DiffStat)
		parts = append(parts, "")
	}

	if len(input.CommitMsgs) > 0 {
		parts = append(parts, "COMMIT MESSAGES:")
		for _, msg := range input.CommitMsgs {
			parts = append(parts, "• "+msg)
		}
		parts = append(parts, "")
	}

	parts = append(parts, "Please provide a thorough PR review covering:")
	parts = append(parts, "- Code quality assessment")
	parts = append(parts, "- Potential issues or concerns")
	parts = append(parts, "- Improvement suggestions")
	parts = append(parts, "- Overall readiness for merge")

	return strings.Join(parts, "\n")
}

// Add missing methods to OpenAI generator
func (g *OpenAIGenerator) buildSynopsisPrompt(input SynopsisInput) string {
	return (&ClaudeGenerator{}).buildSynopsisPrompt(input)
}

func (g *OpenAIGenerator) buildPRReviewPrompt(input PRReviewInput) string {
	return (&ClaudeGenerator{}).buildPRReviewPrompt(input)
}

func (g *OpenAIGenerator) WorkspaceContext(ctx context.Context, input WorkspaceContextInput) (string, error) {
	systemPrompt := "You are a development session assistant helping a developer understand where they left off in their work. Focus on work session continuity, not code quality. This is for 'future me' context passing."
	userPrompt := (&ClaudeGenerator{}).buildWorkspaceContextPrompt(input)
	return g.makeRequest(ctx, systemPrompt, userPrompt)
}

func LoadConfigFromEnv() Config {
	maxTokens := 256
	if val := os.Getenv("WIPCTL_AI_MAX_TOKENS"); val != "" {
		if parsed, err := json.Number(val).Int64(); err == nil {
			maxTokens = int(parsed)
		}
	}

	temperature := 0.1
	if val := os.Getenv("WIPCTL_AI_TEMPERATURE"); val != "" {
		if parsed, err := json.Number(val).Float64(); err == nil {
			temperature = parsed
		}
	}

	return Config{
		Provider:    os.Getenv("WIPCTL_AI_PROVIDER"),
		Endpoint:    os.Getenv("WIPCTL_AI_ENDPOINT"),
		Model:       os.Getenv("WIPCTL_AI_MODEL"),
		Token:       os.Getenv("WIPCTL_AI_TOKEN"),
		ExecPath:    os.Getenv("WIPCTL_AI_EXEC"),
		MaxTokens:   maxTokens,
		Temperature: temperature,
	}
}

func (g *ClaudeGenerator) buildWorkspaceContextPrompt(input WorkspaceContextInput) string {
	return fmt.Sprintf(`🔄 WORKSPACE SESSION BRIEFING 🔄

You're helping a developer understand where they left off. Analyze this workspace state:

OVERVIEW:
- Total Repositories: %d
- Active Repositories: %d
- Repositories with Changes: %d
- Total Files Modified: %d
- Total Lines Changed: %d

REPOSITORY STATUS:
%s

Please provide a briefing that answers:

1. **WORK SESSION SUMMARY**: What was I working on when I stopped?
2. **ACTIVE PROJECTS**: Which repositories have ongoing work?
3. **CURRENT STATE**: What's the status of each active project?
4. **PRIORITY GUIDANCE**: Where should I start when I return?
5. **NEXT STEPS**: What are the logical next actions?

Format your response for a developer returning to work who needs to quickly understand:
- What they were in the middle of
- Which repos need attention
- What the current state means
- Where to pick up development

Focus on actionable context, not code quality assessment.`,
		len(input.Repositories),
		input.ActiveRepos,
		input.DirtyRepos,
		input.TotalFiles,
		input.TotalLines,
		g.formatWorkspaceRepos(input.Repositories))
}

//nolint:unused // TODO: will be used for multi-repo formatting
func (g *ClaudeGenerator) formatRepositories(repos []RepoSummary) string {
	var parts []string
	for _, repo := range repos {
		status := "clean"
		if repo.Status == "dirty" {
			status = "has changes"
		}
		parts = append(parts, fmt.Sprintf("- %s (%s): %s - %d files, +%d/-%d lines, %d commits",
			repo.Name, repo.Branch, status, repo.FilesChanged, repo.LinesAdded, repo.LinesRemoved, repo.Commits))
	}
	return strings.Join(parts, "\n")
}

func (g *ClaudeGenerator) formatWorkspaceRepos(repos []WorkspaceRepo) string {
	var parts []string
	for _, repo := range repos {
		status := "🟢 Clean"
		if repo.Status == "dirty" {
			status = "🟡 Has Changes"
		} else if repo.Status == "in-progress" {
			status = "🔄 In Progress"
		}

		recentWork := "No recent work"
		if len(repo.RecentWork) > 0 {
			recentWork = strings.Join(repo.RecentWork[:min(3, len(repo.RecentWork))], "; ")
		}

		changes := "No changes"
		if repo.Changes != "" {
			changes = repo.Changes
		}

		parts = append(parts, fmt.Sprintf(`
📁 **%s** (%s)
   Status: %s
   Files: %d changed | Lines: +%d/-%d | Commits: %d
   Recent Work: %s
   Current Changes: %s`,
			repo.Name, repo.Branch, status, repo.FilesChanged,
			repo.LinesAdded, repo.LinesRemoved, repo.Commits,
			recentWork, changes))
	}
	return strings.Join(parts, "\n")
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}