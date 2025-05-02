package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os/exec"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
)

// echoMsg is used to receive the output of our ExecProcess callback.
type echoMsg struct {
	Output string
}

type model struct {
	table table.Model
	// submenu state
	showMenu    bool
	menuOptions []string
	menuCursor  int
	menuBranch  string
}

func fetchPRInfo() map[string]string {
	cmd := exec.Command("gh", "pr", "list", "--state", "all", "--json", "headRefName,url,state")
	out, err := cmd.Output()
	if err != nil {
		log.Printf("gh CLI error: %v", err)
		return nil
	}
	var prs []struct {
		HeadRefName string `json:"headRefName"`
		URL         string `json:"url"`
		State       string `json:"state"`
	}
	if err := json.Unmarshal(out, &prs); err != nil {
		log.Printf("JSON parse error: %v", err)
		return nil
	}
	m := make(map[string][]string)
	for _, pr := range prs {
		m[pr.HeadRefName] = append(m[pr.HeadRefName], fmt.Sprintf("%s (%s)", pr.State, pr.URL))
	}
	flat := make(map[string]string, len(m))
	for b, entries := range m {
		flat[b] = strings.Join(entries, ", ")
	}
	return flat
}

func relativeTime(ts int64) string {
	diff := time.Since(time.Unix(ts, 0))
	switch {
	case diff.Hours() > 24*365:
		return fmt.Sprintf("%d years ago", int(diff.Hours()/(24*365)))
	case diff.Hours() > 24*30:
		return fmt.Sprintf("%d months ago", int(diff.Hours()/(24*30)))
	case diff.Hours() > 24:
		return fmt.Sprintf("%d days ago", int(diff.Hours()/24))
	case diff.Hours() > 1:
		return fmt.Sprintf("%d hours ago", int(diff.Hours()))
	case diff.Minutes() > 1:
		return fmt.Sprintf("%d minutes ago", int(diff.Minutes()))
	default:
		return "just now"
	}
}

func initialModel(repoPath string) model {
	// open repo
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		log.Fatalf("open repo %s: %v", repoPath, err)
	}

	// find merged branches
	mergedMap := map[string]bool{}
	if out, err := exec.Command("git", "-C", repoPath, "branch", "--merged").Output(); err == nil {
		for _, ln := range strings.Split(string(out), "\n") {
			name := strings.TrimSpace(strings.TrimPrefix(ln, "* "))
			if name != "" {
				mergedMap[name] = true
			}
		}
	}

	prInfo := fetchPRInfo()

	// author lookup
	authLines, _ := exec.Command("git", "-C", repoPath,
		"for-each-ref", "--format=%(authorname)%00%(refname)").Output()
	authors := make(map[string]string)
	for _, ln := range strings.Split(string(authLines), "\n") {
		parts := strings.SplitN(ln, "\x00", 2)
		if len(parts) == 2 {
			authors[parts[1]] = parts[0]
		}
	}

	// branch configs
	cfg, _ := repo.Config()
	branchCfgs := cfg.Branches

	// gather all refs
	refs, _ := repo.References()
	var rows []table.Row
	refs.ForEach(func(ref *plumbing.Reference) error {
		name := ref.Name().String()
		if name == "HEAD" || strings.HasSuffix(name, "/HEAD") {
			return nil
		}

		short := name
		switch {
		case strings.HasPrefix(name, "refs/heads/"):
			short = strings.TrimPrefix(name, "refs/heads/")
		case strings.HasPrefix(name, "refs/remotes/"):
			short = strings.TrimPrefix(name, "refs/remotes/")
		case strings.HasPrefix(name, "refs/tags/"):
			short = "tag: " + strings.TrimPrefix(name, "refs/tags/")
		case strings.HasPrefix(name, "refs/stash"):
			short = "stash"
		}

		author := authors[name]
		if author == "" {
			author = "Unknown"
		}

		rt := "N/A"
		if bc, ok := branchCfgs[short]; ok && bc.Remote != "" {
			merge := strings.TrimPrefix(string(bc.Merge), "refs/heads/")
			rt = fmt.Sprintf("%s/%s", bc.Remote, merge)
		}

		lu := "?"
		if commit, err := repo.CommitObject(ref.Hash()); err == nil {
			lu = relativeTime(commit.Committer.When.Unix())
		}

		merged := "N/A"
		if strings.HasPrefix(name, "refs/heads/") {
			if mergedMap[short] {
				merged = "Yes"
			} else {
				merged = "No"
			}
		}

		pr := "None"
		if info, ok := prInfo[short]; ok {
			pr = info
		}

		rows = append(rows, table.Row{short, author, rt, lu, merged, pr})
		return nil
	})

	columns := []table.Column{
		{Title: "Ref", Width: 30},
		{Title: "Author", Width: 20},
		{Title: "Tracking", Width: 25},
		{Title: "Updated", Width: 15},
		{Title: "Merged", Width: 8},
		{Title: "PR Info", Width: 50},
	}

	t := table.New(
		table.WithColumns(columns),
		table.WithRows(rows),
		table.WithFocused(true),
		table.WithHeight(20),
	)

	// style
	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("62")).
		BorderBottom(true).
		Bold(true)
	s.Selected = s.Selected.
		Foreground(lipgloss.Color("231")).
		Background(lipgloss.Color("57"))
	t.SetStyles(s)

	return model{
		table:       t,
		menuOptions: []string{"Echo current branch"},
		menuCursor:  0,
	}
}

func (m model) Init() tea.Cmd { return nil }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// handle our echo callback
	if em, ok := msg.(echoMsg); ok {
		// no UI change needed; we could log it if you like:
		log.Printf("echo output: %s", em.Output)
		return m, nil
	}

	// table navigation
	var cmd tea.Cmd
	m.table, cmd = m.table.Update(msg)

	// global quit
	if k, ok := msg.(tea.KeyMsg); ok {
		switch k.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		}
	}

	// submenu active?
	if m.showMenu {
		if k, ok := msg.(tea.KeyMsg); ok {
			switch k.String() {
			case "up", "k":
				if m.menuCursor > 0 {
					m.menuCursor--
				}
			case "down", "j":
				if m.menuCursor < len(m.menuOptions)-1 {
					m.menuCursor++
				}
			case "esc":
				m.showMenu = false
				// change this to shift d
			case "shift+d":
				branch := m.menuBranch
				m.showMenu = false
				// Then we can delete the branch and remote
				return m, tea.ExecProcess(
					exec.Command("echo", branch),
					nil,
					// func(err error, out []byte) tea.Msg {
					// 	if err != nil {
					// 		return echoMsg{Output: err.Error()}
					// 	}
					// 	return echoMsg{Output: string(out)}
					// },
				)
			}
		}
		return m, nil
	}

	// open submenu on Enter
	if k, ok := msg.(tea.KeyMsg); ok && k.String() == "enter" {
		row := m.table.Cursor()
		m.menuBranch = m.table.Rows()[row][0]
		m.showMenu = true
		m.menuCursor = 0
		return m, nil
	}

	return m, cmd
}

func (m model) View() string {
	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("205")).
		Render("🔀  Git Ref Viewer  — press q to quit\n\n")
	out := title + m.table.View()

	if m.showMenu {
		menu := "\n\n" + lipgloss.NewStyle().Bold(true).Render("⮞ Menu:\n")
		for i, opt := range m.menuOptions {
			prefix := "  "
			if i == m.menuCursor {
				prefix = lipgloss.NewStyle().
					Background(lipgloss.Color("57")).
					Foreground(lipgloss.Color("231")).
					Render("> ")
			}
			menu += fmt.Sprintf("%s%s\n", prefix, opt)
		}
		menu += "\n(↑/↓ to move • Enter to select • Esc to cancel)"
		out += menu
	}

	return out
}

func main() {
	repoPath := flag.String("repo-path", ".", "path to Git repo")
	debug := flag.Bool("debug", false, "enable debug logs")
	flag.Parse()

	if !*debug {
		log.SetOutput(nil)
	}

	m := initialModel(*repoPath)
	if err := tea.NewProgram(m, tea.WithAltScreen()).Start(); err != nil {
		fmt.Printf("Error: %v\n", err)
	}
}
