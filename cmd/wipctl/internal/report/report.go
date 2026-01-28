package report

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type ReportEntry struct {
	Repo     string
	Outcome  string
	Details  string
	Warnings []string
	Errors   []string
}

type Report struct {
	Title      string
	Workspace  string
	Timestamp  time.Time
	Entries    []ReportEntry
	reportDir  string
	operation  string
}

func NewReport(title, workspace, reportDir, operation string) *Report {
	return &Report{
		Title:     title,
		Workspace: workspace,
		Timestamp: time.Now(),
		reportDir: reportDir,
		operation: operation,
	}
}

func (r *Report) AddEntry(entry ReportEntry) {
	r.Entries = append(r.Entries, entry)
}

func (r *Report) Save() error {
	if err := os.MkdirAll(r.reportDir, 0755); err != nil {
		return fmt.Errorf("create report directory: %w", err)
	}

	filename := fmt.Sprintf("wip-%s-%s.md", r.operation, r.Timestamp.Format("20060102-150405"))
	filepath := filepath.Join(r.reportDir, filename)

	content := r.generateMarkdown()

	if err := os.WriteFile(filepath, []byte(content), 0644); err != nil {
		return fmt.Errorf("write report file: %w", err)
	}

	return nil
}

func (r *Report) generateMarkdown() string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("# %s\n\n", r.Title))
	sb.WriteString(fmt.Sprintf("**Workspace:** %s  \n", r.Workspace))
	sb.WriteString(fmt.Sprintf("**Timestamp:** %s  \n\n", r.Timestamp.Format(time.RFC3339)))

	if len(r.Entries) == 0 {
		sb.WriteString("No repositories processed.\n")
		return sb.String()
	}

	sb.WriteString("## Results\n\n")

	for _, entry := range r.Entries {
		sb.WriteString(fmt.Sprintf("- **%s**: %s", entry.Repo, entry.Outcome))

		if entry.Details != "" {
			sb.WriteString(fmt.Sprintf(" - %s", entry.Details))
		}

		sb.WriteString("\n")

		for _, warning := range entry.Warnings {
			sb.WriteString(fmt.Sprintf("  ⚠ %s\n", warning))
		}

		for _, error := range entry.Errors {
			sb.WriteString(fmt.Sprintf("  ❌ %s\n", error))
		}
	}

	sb.WriteString("\n")
	return sb.String()
}

func ListReports(reportDir string) ([]string, error) {
	files, err := os.ReadDir(reportDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, fmt.Errorf("read report directory: %w", err)
	}

	var reports []string
	for _, file := range files {
		if !file.IsDir() && strings.HasPrefix(file.Name(), "wip-") && strings.HasSuffix(file.Name(), ".md") {
			reports = append(reports, filepath.Join(reportDir, file.Name()))
		}
	}

	return reports, nil
}

func CreatePushEntry(repo, branch, wipBranch, outcome string) ReportEntry {
	details := ""
	if wipBranch != "" {
		details = fmt.Sprintf("%s (wip=%s)", branch, wipBranch)
	} else {
		details = branch
	}

	return ReportEntry{
		Repo:    repo,
		Outcome: outcome,
		Details: details,
	}
}

func CreatePullEntry(repo, fromBranch, toBranch, outcome string) ReportEntry {
	details := ""
	if fromBranch != "" && toBranch != "" {
		details = fmt.Sprintf("%s → %s", fromBranch, toBranch)
	}

	return ReportEntry{
		Repo:    repo,
		Outcome: outcome,
		Details: details,
	}
}

func (e *ReportEntry) AddWarning(warning string) {
	e.Warnings = append(e.Warnings, warning)
}

func (e *ReportEntry) AddError(error string) {
	e.Errors = append(e.Errors, error)
}