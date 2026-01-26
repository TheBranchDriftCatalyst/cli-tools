package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

/* ============================== Config / Flags ============================== */

type ColorMode int
const ( ColorAuto ColorMode = iota; ColorAlways; ColorNever )

type LogLevel int
const ( LogWarn LogLevel = iota; LogInfo; LogDebug; LogTrace )

type Config struct {
	Remote    string
	AuthorRe  string
	Only      string // all|local|remote
	ColorMode ColorMode
	LogLevel  LogLevel
}

func parseFlags() (Config, bool /*showHelp*/) {
	remote := flag.String("remote", "origin", "Remote name")
	author := flag.String("author", "", "Filter by author email (regex, case-insensitive)")
	locals := flag.Bool("locals-only", false, "Show only local branches")
	remotes := flag.Bool("remotes-only", false, "Show only remote branches")
	colorStr := flag.String("color", "auto", "Color: auto|always|never")
	logStr   := flag.String("log-level", "info", "Log level: warn|info|debug|trace")
	debug    := flag.Bool("debug", false, "Shortcut for --log-level=debug")
	trace    := flag.Bool("trace", false, "Shortcut for --log-level=trace")
	help     := flag.Bool("help", false, "Show help")

	flag.Parse()

	only := "all"
	if *locals && *remotes {
		// both -> all
	} else if *locals {
		only = "local"
	} else if *remotes {
		only = "remote"
	}

	var cm ColorMode
	switch strings.ToLower(*colorStr) {
	case "always": cm = ColorAlways
	case "never": cm = ColorNever
	default: cm = ColorAuto
	}

	var ll LogLevel
	switch {
	case *trace: ll = LogTrace
	case *debug: ll = LogDebug
	default:
		switch strings.ToLower(*logStr) {
		case "trace": ll = LogTrace
		case "debug": ll = LogDebug
		case "warn":  ll = LogWarn
		default:      ll = LogInfo
		}
	}

	return Config{
		Remote:    *remote,
		AuthorRe:  *author,
		Only:      only,
		ColorMode: cm,
		LogLevel:  ll,
	}, *help
}

/* ============================== Structured Logging ========================== */

type logger struct {
	level LogLevel
	buf   ring // in-memory ring buffer for debug pane
}

func newLogger(level LogLevel, cap int) *logger { return &logger{level: level, buf: ring{cap: cap}} }
func (l *logger) SetLevel(level LogLevel) { l.level = level }
func (l *logger) Level() LogLevel         { return l.level }

func (l *logger) logf(level LogLevel, fmtstr string, a ...any) {
	if level > l.level { return }
	ts := time.Now().Format("15:04:05.000")
	lab := map[LogLevel]string{LogWarn:"WARN", LogInfo:"INFO", LogDebug:"DEBUG", LogTrace:"TRACE"}[level]
	line := fmt.Sprintf("%s [%s] %s", ts, lab, fmt.Sprintf(fmtstr, a...))
	l.buf.add(line)
	// Also mirror to stderr when debugging hard
	if l.level >= LogDebug {
		fmt.Fprintln(os.Stderr, line)
	}
}
func (l *logger) Warnf(f string, a ...any)  { l.logf(LogWarn, f, a...) }
func (l *logger) Infof(f string, a ...any)  { l.logf(LogInfo, f, a...) }
func (l *logger) Debugf(f string, a ...any) { l.logf(LogDebug, f, a...) }
func (l *logger) Tracef(f string, a ...any) { l.logf(LogTrace, f, a...) }

type ring struct {
	items []string
	head  int
	full  bool
	cap   int
}

func (r *ring) add(s string) {
	if r.items == nil { r.items = make([]string, r.cap) }
	r.items[r.head] = s
	r.head = (r.head + 1) % r.cap
	if r.head == 0 { r.full = true }
}
func (r *ring) slice() []string {
	if r.items == nil { return nil }
	if !r.full {
		return append([]string(nil), r.items[:r.head]...)
	}
	// head..end + 0..head-1
	out := append([]string(nil), r.items[r.head:]...)
	out = append(out, r.items[:r.head]...)
	return out
}

/* ============================== Domain types ================================ */

type Row struct {
	Branch          string
	Scope           string // local|remote
	Upstream        string // "-" for remote or missing
	UpstreamISO     string // hidden for sorting
	UpstreamHuman   string
	Merged          string // yes|no|-
	PR              string // "#123" or "-"
	PRState         string // open|closed|merged|-
	LastISO         string // hidden for sorting
	LastHuman       string
	Email           string
}

type PRInfo struct{ Number int; State, HeadRef string }

type FetchOpts struct {
	Remote   string
	Only     string // all|local|remote
	AuthorRe *regexp.Regexp
}

/* ============================== Exec helpers with tracing =================== */

func runLogged(ctx context.Context, log *logger, name string, args ...string) (stdout string, stderr string, err error) {
	start := time.Now()
	cmd := exec.CommandContext(ctx, name, args...)
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	log.Tracef("exec: %s %s", name, strings.Join(args, " "))
	err = cmd.Run()
	dur := time.Since(start)
	stdout, stderr = out.String(), errb.String()
	if err != nil {
		log.Debugf("exec: %s %s -> err=%v dur=%s stderr=%q", name, strings.Join(args, " "), err, dur, truncate(stderr, 400))
	} else {
		log.Tracef("exec: %s %s -> ok dur=%s", name, strings.Join(args, " "), dur)
	}
	return
}

/* ============================== Git plumbing =============================== */

const unitSep = '\x1f'

func detectBase(ctx context.Context, log *logger, remote string) (base, baseRef string) {
	if s, _, _ := runLogged(ctx, log, "git", "symbolic-ref", "-q", "refs/remotes/"+remote+"/HEAD"); s != "" {
		s = strings.TrimSpace(s)
		base = strings.TrimPrefix(s, "refs/remotes/"+remote+"/")
	}
	if base == "" {
		if _, _, err := runLogged(ctx, log, "git", "show-ref", "--verify", "refs/remotes/"+remote+"/main"); err == nil { base = "main" } else
		if _, _, err := runLogged(ctx, log, "git", "show-ref", "--verify", "refs/remotes/"+remote+"/master"); err == nil { base = "master" } else
		if cur, _, err := runLogged(ctx, log, "git", "rev-parse", "--abbrev-ref", "HEAD"); err == nil { base = strings.TrimSpace(cur) }
	}
	if _, _, err := runLogged(ctx, log, "git", "show-ref", "--verify", "refs/remotes/"+remote+"/"+base); err == nil {
		baseRef = "refs/remotes/"+remote+"/"+base
	} else if _, _, err := runLogged(ctx, log, "git", "show-ref", "--verify", "refs/heads/"+base); err == nil {
		baseRef = "refs/heads/"+base
	}
	if baseRef == "" {
		log.Warnf("base '%s' not found on %s or locally; MERGED will be '-'", base, remote)
	}
	log.Infof("base: %s (ref: %s)", base, firstNonEmpty(baseRef, "N/A"))
	return
}

func refDateMap(ctx context.Context, log *logger) map[string]string {
	out, _, _ := runLogged(ctx, log, "git", "for-each-ref", "--format=%(refname)\t%(committerdate:iso-strict)", "refs/heads", "refs/remotes")
	m := map[string]string{}
	sc := bufio.NewScanner(strings.NewReader(out))
	for sc.Scan() {
		line := sc.Text()
		parts := strings.SplitN(line, "\t", 2)
		if len(parts) == 2 { m[parts[0]] = parts[1] }
	}
	log.Tracef("refDateMap loaded: %d", len(m))
	return m
}

func collectRefRows(ctx context.Context, log *logger, remote, only string) ([][]string, error) {
	format := "%(refname)\x1f%(refname:short)\x1f%(upstream:short)\x1f%(committerdate:iso-strict)\x1f%(authoremail)"
	args := []string{"for-each-ref", "--sort=-committerdate", "--format=" + format}
	switch only {
	case "local": args = append(args, "refs/heads")
	case "remote": args = append(args, "refs/remotes")
	default: args = append(args, "refs/heads", "refs/remotes")
	}
	out, _, err := runLogged(ctx, log, "git", args...)
	if err != nil { return nil, err }
	var rows [][]string
	sc := bufio.NewScanner(strings.NewReader(out))
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, "refs/remotes/"+remote+"/HEAD") { continue }
		parts := strings.Split(line, string(unitSep))
		if len(parts) != 5 { continue }
		rows = append(rows, parts)
	}
	log.Tracef("collectRefRows: %d", len(rows))
	return rows, nil
}

func loadPRs(ctx context.Context, log *logger) map[string]PRInfo {
	if _, err := exec.LookPath("gh"); err != nil {
		log.Warnf("gh not available; PR columns will be '-'")
		return nil
	}
	out, _, err := runLogged(ctx, log, "gh", "pr", "list", "--state", "all", "--json", "number,state,headRefName", "--limit", "1000")
	if err != nil || strings.TrimSpace(out) == "" { log.Warnf("gh pr list failed/empty; PRs '-'"); return nil }
	var prs []struct{ Number int; State, HeadRefName string }
	if json.Unmarshal([]byte(out), &prs) != nil { log.Warnf("failed to decode gh json; PRs '-'"); return nil }
	mp := make(map[string]PRInfo, len(prs))
	for _, p := range prs { mp[p.HeadRefName] = PRInfo{Number: p.Number, State: strings.ToLower(p.State), HeadRef: p.HeadRefName} }
	log.Infof("loaded %d PRs via gh", len(prs))
	return mp
}

/* ============================== Data transform ============================== */

func collectRows(ctx context.Context, log *logger, cfg Config) ([]Row, string /*base*/, string /*baseRef*/, error) {
	if err := exec.Command("git", "rev-parse", "--git-dir").Run(); err != nil {
		return nil, "", "", errors.New("not a git repo")
	}
	base, baseRef := detectBase(ctx, log, cfg.Remote)
	refdates := refDateMap(ctx, log)
	rrefs, err := collectRefRows(ctx, log, cfg.Remote, cfg.Only)
	if err != nil { return nil, "", "", err }

	var re *regexp.Regexp
	if cfg.AuthorRe != "" {
		re, err = regexp.Compile("(?i)" + cfg.AuthorRe)
		if err != nil { return nil, "", "", fmt.Errorf("invalid --author regex: %v", err) }
	}

	prs := loadPRs(ctx, log)

	rows := make([]Row, 0, len(rrefs))
	for _, parts := range rrefs {
		ref, short, upstream, lastISO, email := parts[0], parts[1], parts[2], parts[3], parts[4]
		scope := "remote"; if strings.HasPrefix(ref, "refs/heads/") { scope = "local" }
		if re != nil && !re.MatchString(email) { continue }

		up := upstream
		if scope == "remote" || up == "" { up = "-" }

		merged := "-"
		if baseRef != "" && short != base && ref != baseRef {
			if exec.Command("git", "merge-base", "--is-ancestor", ref, baseRef).Run() == nil { merged = "yes" } else { merged = "no" }
		}

		prNo, prState := "-", "-"
		if prs != nil {
			if pr, ok := prs[short]; ok {
				prNo, prState = fmt.Sprintf("#%d", pr.Number), pr.State
			} else if i := strings.IndexByte(short, '/'); i > 0 {
				if pr, ok := prs[short[i+1:]]; ok { prNo, prState = fmt.Sprintf("#%d", pr.Number), pr.State }
			}
		}

		usISO := ""
		upHuman := "-"
		if up != "-" {
			if d, ok := refdates["refs/remotes/"+up]; ok { usISO = d } else
			if d, ok := refdates["refs/heads/"+up]; ok { usISO = d }
			if usISO != "" { upHuman = relHuman(usISO) }
		}

		rows = append(rows, Row{
			Branch: short, Scope: scope, Upstream: up,
			UpstreamISO: usISO, UpstreamHuman: upHuman,
			Merged: merged, PR: prNo, PRState: prState,
			LastISO: lastISO, LastHuman: relHuman(lastISO),
			Email: email,
		})
	}
	return rows, base, baseRef, nil
}

/* ============================== Relative time =============================== */

func relHuman(iso string) string {
	if iso == "" { return "-" }
	t, err := time.Parse(time.RFC3339, iso)
	if err != nil {
		if i := strings.IndexByte(iso, 'T'); i > 0 { return iso[:i] }
		return iso
	}
	d := time.Since(t); if d < 0 { d = -d }
	min, hr, day := time.Minute, time.Hour, 24*time.Hour
	week, month, year := 7*day, 30*day, 365*day
	switch {
	case d < min: return "just now"
	case d < hr:  return plural(int(d/min), "min")
	case d < day: return plural(int(d/hr), "hour")
	case d < week: return plural(int(d/day), "day")
	case d < month: return plural(int(d/week), "week")
	case d < year: return plural(int(d/month), "month")
	default: return plural(int(d/year), "year")
	}
}
func plural(n int, u string) string {
	if n == 1 { return fmt.Sprintf("%d %s ago", n, u) }
	return fmt.Sprintf("%d %ss ago", n, u)
}

/* ============================== TUI ======================================== */

type sortKey int
const (
	colBranch sortKey = iota; colScope; colUpstream; colUpUpdated; colMerged; colPR; colPRState; colLast; colEmail
)

func sortRows(rows []Row, key sortKey, desc bool) []Row {
	cp := make([]Row, len(rows)); copy(cp, rows)
	less := func(i, j int) bool { return false }
	switch key {
	case colBranch: less = func(i, j int) bool { return cp[i].Branch < cp[j].Branch }
	case colScope:  less = func(i, j int) bool { return cp[i].Scope < cp[j].Scope }
	case colUpstream: less = func(i, j int) bool { return cp[i].Upstream < cp[j].Upstream }
	case colUpUpdated: less = func(i, j int) bool { return cp[i].UpstreamISO < cp[j].UpstreamISO }
	case colMerged:
		rank := func(s string) int { switch s {case "yes": return 0; case "no": return 1; default: return 2} }
		less = func(i, j int) bool { return rank(cp[i].Merged) < rank(cp[j].Merged) }
	case colPR:
		num := func(s string) int { if strings.HasPrefix(s, "#") { n := strings.TrimPrefix(s, "#"); v,_ := strconv.Atoi(n); return v }; return -1 }
		less = func(i, j int) bool { return num(cp[i].PR) < num(cp[j].PR) }
	case colPRState: less = func(i, j int) bool { return cp[i].PRState < cp[j].PRState }
	case colLast:    less = func(i, j int) bool { return cp[i].LastISO < cp[j].LastISO }
	case colEmail:   less = func(i, j int) bool { return cp[i].Email < cp[j].Email }
	}
	sort.Slice(cp, func(i, j int) bool { if desc { return !less(i,j) } ; return less(i,j) })
	return cp
}

type model struct {
	cfg      Config
	log      *logger
	rows     []Row
	base     string
	baseRef  string

	tbl       table.Model
	filter    string
	sortCol   sortKey
	desc      bool
	status    string

	showDebug bool
	lastDump  string // last log dump path

	// styles
	styleHdr lipgloss.Style
	styleSel lipgloss.Style
}

func newModel(cfg Config, log *logger, rows []Row, base, baseRef string, color ColorMode) model {
	columns := []table.Column{
		{Title: "BRANCH", Width: 26}, {Title: "SCOPE", Width: 8},
		{Title: "UPSTREAM", Width: 22}, {Title: "UPSTREAM_UPDATED", Width: 18},
		{Title: "MERGED", Width: 8}, {Title: "PR", Width: 8},
		{Title: "PR_STATE", Width: 10}, {Title: "LAST_UPDATE", Width: 16},
		{Title: "EMAIL", Width: 28},
	}
	t := table.New(table.WithColumns(columns), table.WithFocused(true), table.WithHeight(18))

	hdr := lipgloss.NewStyle()
	sel := lipgloss.NewStyle().Foreground(lipgloss.Color("0")).Background(lipgloss.Color("201"))
	if useColor(color) {
		hdr = lipgloss.NewStyle().Foreground(lipgloss.Color("201")).Bold(true)
	}

	m := model{
		cfg: cfg, log: log, rows: rows, base: base, baseRef: baseRef,
		tbl: t, sortCol: colLast, desc: true,
		styleHdr: hdr, styleSel: sel,
	}
	m.apply()
	return m
}

func useColor(cm ColorMode) bool {
	switch cm {
	case ColorAlways: return true
	case ColorNever:  return false
	default:
		fi, _ := os.Stdout.Stat()
		if (fi.Mode() & os.ModeCharDevice) == 0 { return false }
		if _, err := exec.LookPath("tput"); err != nil { return true }
		out, _ := exec.Command("tput", "colors").Output()
		n, _ := strconv.Atoi(strings.TrimSpace(string(out)))
		return n >= 8
	}
}

func (m *model) apply() {
	cur := sortRows(applyFilter(m.rows, m.filter), m.sortCol, m.desc)
	data := make([]table.Row, len(cur))
	for i, r := range cur {
		scope := r.Scope
		if useColor(m.cfg.ColorMode) {
			if scope == "local" { scope = lipgloss.NewStyle().Foreground(lipgloss.Color("51")).Render(scope) } else {
				scope = lipgloss.NewStyle().Foreground(lipgloss.Color("141")).Render(scope)
			}
		}
		merged := r.Merged
		if useColor(m.cfg.ColorMode) {
			switch merged {
			case "yes": merged = lipgloss.NewStyle().Foreground(lipgloss.Color("82")).Render("yes")
			case "no":  merged = lipgloss.NewStyle().Foreground(lipgloss.Color("208")).Render("no")
			}
		}
		ps := r.PRState
		if useColor(m.cfg.ColorMode) {
			switch ps {
			case "open":   ps = lipgloss.NewStyle().Foreground(lipgloss.Color("82")).Render(ps)
			case "merged": ps = lipgloss.NewStyle().Foreground(lipgloss.Color("45")).Render(ps)
			case "closed": ps = lipgloss.NewStyle().Foreground(lipgloss.Color("208")).Render(ps)
			}
		}
		data[i] = table.Row{r.Branch, scope, r.Upstream, r.UpstreamHuman, merged, r.PR, ps, r.LastHuman, r.Email}
	}
	m.tbl.SetRows(data)

	m.status = fmt.Sprintf("base: %s  rows: %d  time: %s  level: %s",
		m.base, len(m.rows), time.Now().Format("15:04:05"),
		map[LogLevel]string{LogWarn:"warn", LogInfo:"info", LogDebug:"debug", LogTrace:"trace"}[m.log.level],
	)
}

func applyFilter(rows []Row, q string) []Row {
	if strings.TrimSpace(q) == "" { return rows }
	q = strings.ToLower(q)
	out := make([]Row, 0, len(rows))
	for _, r := range rows {
		if strings.Contains(strings.ToLower(r.Branch), q) ||
		   strings.Contains(strings.ToLower(r.Email), q) ||
		   strings.Contains(strings.ToLower(r.Upstream), q) {
			out = append(out, r)
		}
	}
	return out
}

/* ============================== Messages / Cmds ============================= */

type msgRefresh struct{ rows []Row; base, baseRef string; err error }
type msgDumped  struct{ path string; err error }

func refreshCmd(cfg Config, log *logger) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second); defer cancel()
		rows, base, baseRef, err := collectRows(ctx, log, cfg)
		return msgRefresh{rows: rows, base: base, baseRef: baseRef, err: err}
	}
}

func dumpLogsCmd(log *logger) tea.Cmd {
	return func() tea.Msg {
		path := filepath.Join(os.TempDir(), fmt.Sprintf("branchclean-%d.log", time.Now().Unix()))
		f, err := os.Create(path)
		if err != nil { return msgDumped{path:"", err:err} }
		defer f.Close()
		for _, line := range log.buf.slice() {
			_, _ = f.WriteString(line + "\n")
		}
		return msgDumped{path:path, err:nil}
	}
}

/* ============================== Bubble Tea ================================= */

func (m model) Init() tea.Cmd { return refreshCmd(m.cfg, m.log) }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case msgRefresh:
		if msg.err != nil {
			m.status = "error: " + msg.err.Error()
		} else {
			m.rows, m.base, m.baseRef = msg.rows, msg.base, msg.baseRef
			m.apply()
			m.log.Infof("refreshed: %d rows", len(m.rows))
		}
	case msgDumped:
		if msg.err != nil {
			m.status = "log dump failed: " + msg.err.Error()
		} else {
			m.lastDump = msg.path
			m.status = "logs dumped to: " + msg.path
		}
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "up", "k":
			m.tbl.MoveUp(1)
		case "down", "j":
			m.tbl.MoveDown(1)
		case "pgup":
			m.tbl.MoveUp(10)
		case "pgdown":
			m.tbl.MoveDown(10)
		case "r":
			return m, refreshCmd(m.cfg, m.log)
		case "/":
			// crude prompt via env var + read (kept small). Swap to bubbles/textinput if you want proper inline input.
			m.status = "filter: type and press Enter (Esc clears) — not interactive here; set FILTER env then press r"
		case "s":
			m.sortCol = (m.sortCol + 1) % 9
			m.apply()
		case "S":
			m.desc = !m.desc
			m.apply()
		case "d":
			m.showDebug = !m.showDebug
		case "D":
			// cycle log level
			switch m.log.level {
			case LogWarn:  m.log.level = LogInfo
			case LogInfo:  m.log.level = LogDebug
			case LogDebug: m.log.level = LogTrace
			default:       m.log.level = LogWarn
			}
			m.apply()
		case "L":
			return m, dumpLogsCmd(m.log)
		}
	}
	return m, nil
}

func (m model) View() string {
	header := m.styleHdr.Render(
		"↑/↓ nav  s/S sort  r refresh  d debug-pane  D cycle-level  L dump-logs  q quit",
	)
	body := m.tbl.View()
	status := "\n " + m.status

	if m.showDebug {
		logLines := m.log.buf.slice()
		if len(logLines) == 0 {
			logLines = []string{"<no debug lines yet>"}
		}
		max := 10
		if len(logLines) > max {
			logLines = logLines[len(logLines)-max:]
		}
		debugPane := strings.Join(logLines, "\n")
		if useColor(m.cfg.ColorMode) {
			debugPane = lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Render(debugPane)
		}
		return lipgloss.JoinVertical(lipgloss.Left, header, body, status, "\n── debug ──\n"+debugPane)
	}

	return lipgloss.JoinVertical(lipgloss.Left, header, body, status)
}

/* ============================== Main ======================================= */

func main() {
	cfg, help := parseFlags()
	if help {
		flag.Usage()
		return
	}
	log := newLogger(cfg.LogLevel, 200)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	rows, base, baseRef, err := collectRows(ctx, log, cfg)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	log.Infof("startup: rows=%d base=%s baseRef=%s", len(rows), base, baseRef)

	m := newModel(cfg, log, rows, base, baseRef, cfg.ColorMode)
	if err := tea.NewProgram(m).Start(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

/* ============================== Utils ====================================== */

func firstNonEmpty(a, b string) string {
	if a != "" { return a }
	return b
}

func truncate(s string, n int) string {
	if n <= 0 || len(s) <= n { return s }
	// keep rune safety
	rs := []rune(s)
	if len(rs) <= n { return s }
	return string(rs[:n]) + "…"
}
