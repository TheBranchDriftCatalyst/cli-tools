package ui

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/pterm/pterm"
)

// 🔥 CYBERPUNK SYNTHWAVE TERMINAL UI 🔥
// Pure PTerm implementation - no legacy bullshit

// CyberpunkBanner displays the main application banner
func Banner(title string) {
	CyberpunkBanner(title)
}

func CyberpunkBanner(title string) {
	// 🔥 SICK CYBERPUNK BANNER 🔥
	pterm.DefaultCenter.WithCenterEachLineSeparately().Println(
		pterm.DefaultBox.
			WithTitle("WORKSPACE INTELLIGENCE PLATFORM").
			WithTitleTopCenter().
			WithBoxStyle(&pterm.Style{pterm.FgMagenta}).
			Sprint(pterm.LightCyan("▶ "+title+" ◀")),
	)
	pterm.Println()
}

// 🔥 MODERN TABLE SYSTEM 🔥
var (
	tableData    [][]string
	tableHeaders []string
)

// InitTable creates a new modern table
func InitTable(headers ...string) {
	tableData = [][]string{}
	tableHeaders = make([]string, len(headers))

	// Add cyberpunk symbols and styling to headers
	for i, header := range headers {
		styled := addHeaderSymbol(header)
		tableHeaders[i] = pterm.FgCyan.Sprint(pterm.Bold.Sprint(styled))
	}
}

// AddTableRow adds data to the table
func AddTableRow(values ...string) {
	tableData = append(tableData, values)
}

// RenderTable displays the completed table
func RenderTable() {
	if len(tableData) == 0 {
		Warning("No data to display")
		return
	}

	// Create PTerm table data
	ptermData := pterm.TableData{}
	ptermData = append(ptermData, tableHeaders)

	for _, row := range tableData {
		ptermData = append(ptermData, row)
	}

	// 🔥 RENDER BEAUTIFUL TABLE 🔥
	pterm.DefaultTable.
		WithHasHeader(true).
		WithBoxed(false).
		WithLeftAlignment().
		WithSeparator("  ").
		WithRowSeparator("").
		WithData(ptermData).
		Render() //nolint:errcheck // Table rendering errors are non-critical

	pterm.Println()

	// Reset for next table
	tableData = [][]string{}
	tableHeaders = []string{}
}

// addHeaderSymbol adds cyberpunk symbols to headers
func addHeaderSymbol(header string) string {
	switch strings.ToLower(header) {
	case "repository", "repo":
		return "⬢ " + strings.ToUpper(header)
	case "branch":
		return "⌬ " + strings.ToUpper(header)
	case "status":
		return "◆ " + strings.ToUpper(header)
	case "files":
		return "📁 " + strings.ToUpper(header)
	case "lines":
		return "⟨⟩ " + strings.ToUpper(header)
	case "commits":
		return "⬡ " + strings.ToUpper(header)
	case "ahead":
		return "▲ " + strings.ToUpper(header)
	case "behind":
		return "▼ " + strings.ToUpper(header)
	case "size":
		return "💾 " + strings.ToUpper(header)
	default:
		return "● " + strings.ToUpper(header)
	}
}

// 🔥 STATUS STYLING FUNCTIONS 🔥

// StatusCell returns styled status indicators
func StatusCell(status string) string {
	switch status {
	case "clean":
		return pterm.FgGreen.Sprint("✓ CLEAN")
	case "dirty":
		return pterm.FgYellow.Sprint("⚠ DIRTY")
	case "error":
		return pterm.FgRed.Sprint("✗ ERROR")
	case "no-origin":
		return pterm.FgLightYellow.Sprint("⊘ NO-REMOTE")
	case "in-progress":
		return pterm.FgLightMagenta.Sprint("⟳ IN-PROGRESS")
	default:
		return status
	}
}

// CyberText applies cyberpunk styling to text
func CyberText(text string, textType string) string {
	switch textType {
	case "repo":
		return pterm.FgCyan.Sprint(text)
	case "branch":
		return pterm.FgYellow.Sprint(text)
	case "commit":
		return pterm.FgLightWhite.Sprint(text)
	case "size":
		return pterm.FgMagenta.Sprint(text)
	default:
		return text
	}
}

// SynthwaveNumber formats numbers with glow effect
func SynthwaveNumber(n int, colorType string) string {
	if n == 0 {
		return pterm.FgGray.Sprint("—")
	}

	numStr := fmt.Sprintf("%d", n)
	switch colorType {
	case "files":
		return pterm.FgGreen.Sprint("[" + numStr + "]")
	case "commits":
		return pterm.FgYellow.Sprint("[" + numStr + "]")
	case "ahead":
		return pterm.FgGreen.Sprint("[" + numStr + "]")
	case "behind":
		return pterm.FgRed.Sprint("[" + numStr + "]")
	default:
		return pterm.FgWhite.Sprint("[" + numStr + "]")
	}
}

// 🔥 MESSAGE FUNCTIONS 🔥

func Info(msg string) {
	pterm.Info.Println(msg)
}

func Success(msg string) {
	pterm.Success.Println(msg)
}

func Warning(msg string) {
	pterm.Warning.Println(msg)
}

func Error(msg string) {
	pterm.Error.Println(msg)
}

// 🔥 INPUT FUNCTIONS 🔥

func Confirm(question string) bool {
	fmt.Printf("%s [y/N] ", question)
	reader := bufio.NewReader(os.Stdin)
	answer, _ := reader.ReadString('\n')
	answer = strings.TrimSpace(answer)
	return strings.ToLower(answer) == "y" || strings.ToLower(answer) == "yes"
}

// Legacy functions removed - use InitTable/AddTableRow/RenderTable directly