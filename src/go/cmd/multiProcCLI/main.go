package multiProcCLI

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	ui "github.com/gizak/termui/v3"
	"github.com/gizak/termui/v3/widgets"
)

type Process struct {
	Name    string
	Cmd     *exec.Cmd
	Status  string
	LogText *widgets.List
	LogChan chan string
	ErrChan chan string
	Mutex   sync.Mutex
	Running bool
}

var processes []*Process
var tabPane *widgets.TabPane
var logDisplay *widgets.List // Secondary display for logs
var globalMutex sync.Mutex

var autoScroll bool = true

func main() {
	if len(os.Args) < 2 {
		log.Fatalf("Usage: %s <command1> <command2> ...", os.Args[0])
	}

	if err := ui.Init(); err != nil {
		log.Fatalf("failed to initialize termui: %v", err)
	}
	defer ui.Close()

	processes = setupProcesses(os.Args[1:])
	grid := createGrid()
	initializeUI()

	uiEvents := ui.PollEvents()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT, syscall.SIGABRT)

	for {
		select {
		case <-sigChan:
			cleanup()
			ticker.Stop()
			return
		case e := <-uiEvents:
			if handleEvent(e) { // Modified to check if we should quit
				cleanup()
				ticker.Stop()
				return
			}
			renderActiveTab()
		case <-ticker.C:
			renderActiveTab()
		}
		ui.Render(grid)
	}
}

func cleanup() {
	for _, proc := range processes {
		proc.Running = false // Set running to false to stop goroutines
		if proc.Cmd != nil && proc.Cmd.Process != nil {
			proc.Cmd.Process.Kill() // Ensure each process is killed
		}
		// Wait for the process's goroutines to finish (captureOutput)
		time.Sleep(100 * time.Millisecond) // Give some time for goroutines to exit
	}
	// Close channels after ensuring no more data is being sent
	// for _, proc := range processes {
	// 	close(proc.LogChan)
	// 	close(proc.ErrChan)
	// }
	ui.Close() // Close the UI cleanly
}

func createGrid() *ui.Grid {
	grid := ui.NewGrid()
	termWidth, termHeight := ui.TerminalDimensions()
	grid.SetRect(0, 0, termWidth, termHeight)

	helpBar := widgets.NewParagraph()
	helpBar.Text = "Press 'q' to quit, '<Left>' and '<Right>' to switch tabs, 'j' and 'k' to scroll, 'g' and 'G' to go to top and bottom"

	tabPane = widgets.NewTabPane()
	tabPane.Border = true
	tabPane.ActiveTabStyle = ui.NewStyle(ui.ColorBlack, ui.ColorGreen, ui.ModifierUnderline)
	tabPane.InactiveTabStyle = ui.NewStyle(ui.ColorBlack, ui.ColorClear)
	tabPane.Block.Title = "Processes"

	logDisplay = widgets.NewList()
	logDisplay.Border = true
	logDisplay.SelectedRowStyle = ui.NewStyle(ui.ColorYellow)
	logDisplay.WrapText = true
	logDisplay.Title = "Logs"

	grid.Set(
		ui.NewRow(.1, tabPane),
		ui.NewRow(0.7, logDisplay),
		ui.NewRow(.15, helpBar),
	)

	return grid
}

func handleEvent(e ui.Event) bool {
	switch e.ID {
	case "q", "<C-c>":
		return true
	case "<Left>", "l":
		tabPane.FocusLeft()
	case "<Right>", "j":
		tabPane.FocusRight()
	case "k", "<Down>":
		autoScroll = false
		logDisplay.ScrollDown()
	case "i", "<Up>":
		autoScroll = false
		logDisplay.ScrollUp()
	case "<C-d>":
		autoScroll = false
		logDisplay.ScrollHalfPageDown()
	case "<C-u>":
		autoScroll = false
		logDisplay.ScrollHalfPageUp()
	case "<C-f>":
		autoScroll = false
		logDisplay.ScrollPageDown()
	case "<C-b>":
		autoScroll = false
		logDisplay.ScrollPageUp()
	case "g", "<Home>":
		autoScroll = false
		logDisplay.ScrollTop()
	case "G", "<End>":
		logDisplay.ScrollBottom()
		autoScroll = true
	}
	return false
}

func setupProcesses(commands []string) []*Process {
	processes := make([]*Process, len(commands))
	for i, cmd := range commands {
		parts := strings.Fields(cmd) // Splits the command into all parts
		processes[i] = &Process{
			Name:    strings.Join(parts, ""),
			Cmd:     exec.Command(parts[0], parts[1:]...), // parts[1:] will correctly pass all arguments and flags
			Status:  "Starting",
			LogText: widgets.NewList(),
			LogChan: make(chan string, 100),
			ErrChan: make(chan string, 100),
		}
		processes[i].LogText.Title = "Logs for " + parts[0]
		processes[i].LogText.WrapText = true
		processes[i].Cmd.Env = os.Environ() // Inherit environment
		processes[i].Running = true

		go runProcess(processes[i], i)
	}
	return processes
}

func runProcess(p *Process, index int) {
	stdout, err := p.Cmd.StdoutPipe()
	if err != nil {
		// Try to send to channel, ignore if closed
		select {
		case p.LogChan <- fmt.Sprintf("Error creating stdout pipe: %v", err):
		default:
		}
		p.Mutex.Lock()
		p.Status = "Error"
		p.Mutex.Unlock()
		updateUI(index)
		return
	}

	stderr, err := p.Cmd.StderrPipe()
	if err != nil {
		// Try to send to channel, ignore if closed
		select {
		case p.LogChan <- fmt.Sprintf("Error creating stderr pipe: %v", err):
		default:
		}
		p.Mutex.Lock()
		p.Status = "Error"
		p.Mutex.Unlock()
		updateUI(index)
		return
	}

	go captureOutput(stdout, p.LogChan, p)
	go captureOutput(stderr, p.ErrChan, p)

	if err := p.Cmd.Start(); err != nil {
		// Try to send to channel, ignore if closed
		select {
		case p.LogChan <- fmt.Sprintf("Error starting process: %v", err):
		default:
		}
		p.Mutex.Lock()
		p.Status = "Error"
		p.Mutex.Unlock()
		appendLog(p, err.Error(), true)
		updateUI(index)
		return
	}

	go func() {
		for log := range p.LogChan {
			appendLog(p, log, false)
		}
		for err := range p.ErrChan {
			appendLog(p, err, true)
		}
	}()

	p.Mutex.Lock()
	p.Status = "Running"
	p.Mutex.Unlock()
	updateUI(index)
	if err := p.Cmd.Wait(); err != nil {
		// p.LogChan <- fmt.Sprintf("Process ended with error: %v", err)
		p.Mutex.Lock()
		p.Status = "Error"
		p.Mutex.Unlock()
	}
	updateUI(index)
}

func captureOutput(pipe io.ReadCloser, channel chan<- string, proc *Process) {
	defer func() {
		if r := recover(); r != nil {
			// Channel might already be closed, ignore panic
		}
	}()
	defer close(channel)

	scanner := bufio.NewScanner(pipe)
	for scanner.Scan() { // Check if still running
		proc.Mutex.Lock()
		running := proc.Running
		proc.Mutex.Unlock()
		if !running {
			break
		}
		select {
		case channel <- scanner.Text():
		default:
			// Channel might be closed, ignore
		}
	}
}

func appendLog(p *Process, log string, isError bool) {
	timestamp := time.Now().Format("15:04:05")
	formattedLog := fmt.Sprintf("%s > %s", timestamp, log)
	p.Mutex.Lock()
	p.LogText.Rows = append(p.LogText.Rows, formattedLog)
	p.Mutex.Unlock()
	if autoScroll {
		scrollToLatest(p)
	}
}

func scrollToLatest(p *Process) {
	p.Mutex.Lock()
	defer p.Mutex.Unlock()

	lastRow := len(p.LogText.Rows) - 1
	if lastRow < 0 {
		lastRow = 0
	}
	p.LogText.SelectedRow = lastRow

	if tabPane.ActiveTabIndex >= 0 && processes[tabPane.ActiveTabIndex] == p {
		logDisplay.SelectedRow = lastRow
	}
}

func initializeUI() {
	termWidth, _ := ui.TerminalDimensions()
	tabPane.SetRect(0, 0, termWidth, 3)
	for i := range processes {
		tabPane.TabNames = append(tabPane.TabNames, fmt.Sprintf("%s (?) %s", processes[i].Name, "Starting"))
	}
	ui.Render(tabPane, logDisplay)
}

func updateUI(index int) {
	globalMutex.Lock()
	defer globalMutex.Unlock()

	// Bounds checking to prevent panics during tests
	if index < 0 || index >= len(processes) || tabPane == nil || index >= len(tabPane.TabNames) {
		return
	}

	p := processes[index]

	// Get status with proper locking
	p.Mutex.Lock()
	status := p.Status
	p.Mutex.Unlock()

	if p.Cmd.Process != nil {
		tabPane.TabNames[index] = fmt.Sprintf("%s (%d) %s", p.Name, p.Cmd.Process.Pid, status)
	} else {
		tabPane.TabNames[index] = fmt.Sprintf("%s (?) %s", p.Name, status)
	}
	if index == tabPane.ActiveTabIndex && logDisplay != nil {
		logDisplay.Rows = p.LogText.Rows
		scrollToLatest(p)
	}
}

func renderActiveTab() {
	if tabPane.ActiveTabIndex >= 0 && tabPane.ActiveTabIndex < len(processes) {
		p := processes[tabPane.ActiveTabIndex]
		logDisplay.Rows = p.LogText.Rows
		scrollToLatest(p)
	}
}
