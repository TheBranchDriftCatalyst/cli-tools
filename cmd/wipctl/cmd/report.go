package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/TheBranchDriftCatalyst/cli-tools/cmd/wipctl/internal/report"
	"github.com/TheBranchDriftCatalyst/cli-tools/cmd/wipctl/internal/ui"
)

var reportCmd = &cobra.Command{
	Use:   "report",
	Short: "List and display wipctl operation reports",
	Long: `List all saved wipctl reports from previous push and pull operations.

Reports are stored as Markdown files in the report directory (default: <workspace>/.wipctl).
Each report contains details about the operation performed on each repository,
including outcomes, warnings, and errors.

Use the --show flag to display the contents of a specific report.`,
	RunE: runReport,
}

var showReport string

func init() {
	rootCmd.AddCommand(reportCmd)
	reportCmd.Flags().StringVar(&showReport, "show", "", "show contents of specific report file")
}

func runReport(cmd *cobra.Command, args []string) error {
	reports, err := report.ListReports(reportDir)
	if err != nil {
		ui.Error("Failed to list reports: " + err.Error())
		return err
	}

	if len(reports) == 0 {
		ui.Info("No reports found in " + reportDir)
		return nil
	}

	if showReport != "" {
		return displayReport(showReport)
	}

	displayReportsList(reports)
	return nil
}

func displayReportsList(reports []string) {
	ui.Info(fmt.Sprintf("Found %d reports in %s", len(reports), reportDir))
	fmt.Println()

	reportInfos, err := getReportInfos(reports)
	if err != nil {
		ui.Error("Failed to get report information: " + err.Error())
		return
	}

	sort.Slice(reportInfos, func(i, j int) bool {
		return reportInfos[i].ModTime.After(reportInfos[j].ModTime)
	})

	ui.InitTable("Report File", "Operation", "Age", "Size")

	for _, info := range reportInfos {
		age := formatAge(info.ModTime)
		size := formatSize(info.Size)
		operation := extractOperation(info.Name)

		ui.AddTableRow(
			filepath.Base(info.Name),
			operation,
			age,
			size,
		)
	}

	ui.RenderTable()

	fmt.Printf("\nUse --show <filename> to display a specific report\n")
}

func displayReport(filename string) error {
	var reportPath string

	if filepath.IsAbs(filename) {
		reportPath = filename
	} else {
		reportPath = filepath.Join(reportDir, filename)
	}

	content, err := os.ReadFile(reportPath)
	if err != nil {
		ui.Error("Failed to read report: " + err.Error())
		return err
	}

	ui.Info("Report: " + filepath.Base(reportPath))
	fmt.Println()
	fmt.Print(string(content))

	return nil
}

type reportInfo struct {
	Name    string
	Size    int64
	ModTime time.Time
}

func getReportInfos(reports []string) ([]reportInfo, error) {
	var infos []reportInfo

	for _, reportFile := range reports {
		stat, err := os.Stat(reportFile)
		if err != nil {
			continue
		}

		infos = append(infos, reportInfo{
			Name:    reportFile,
			Size:    stat.Size(),
			ModTime: stat.ModTime(),
		})
	}

	return infos, nil
}

func extractOperation(filename string) string {
	base := filepath.Base(filename)

	if strings.HasPrefix(base, "wip-push-") {
		return "push"
	}
	if strings.HasPrefix(base, "wip-pull-") {
		return "pull"
	}

	parts := strings.Split(base, "-")
	if len(parts) >= 2 {
		return parts[1]
	}

	return "unknown"
}

func formatAge(t time.Time) string {
	duration := time.Since(t)

	switch {
	case duration < time.Minute:
		return "just now"
	case duration < time.Hour:
		minutes := int(duration.Minutes())
		if minutes == 1 {
			return "1 minute ago"
		}
		return fmt.Sprintf("%d minutes ago", minutes)
	case duration < 24*time.Hour:
		hours := int(duration.Hours())
		if hours == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", hours)
	case duration < 7*24*time.Hour:
		days := int(duration.Hours() / 24)
		if days == 1 {
			return "1 day ago"
		}
		return fmt.Sprintf("%d days ago", days)
	case duration < 30*24*time.Hour:
		weeks := int(duration.Hours() / (7 * 24))
		if weeks == 1 {
			return "1 week ago"
		}
		return fmt.Sprintf("%d weeks ago", weeks)
	default:
		months := int(duration.Hours() / (30 * 24))
		if months == 1 {
			return "1 month ago"
		}
		return fmt.Sprintf("%d months ago", months)
	}
}

func formatSize(size int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)

	switch {
	case size < KB:
		return fmt.Sprintf("%dB", size)
	case size < MB:
		return fmt.Sprintf("%.1fKB", float64(size)/KB)
	case size < GB:
		return fmt.Sprintf("%.1fMB", float64(size)/MB)
	default:
		return fmt.Sprintf("%.1fGB", float64(size)/GB)
	}
}