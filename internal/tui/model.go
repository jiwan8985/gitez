// Package tui implements a full-screen terminal UI for gez using bubbletea.
package tui

import (
	"fmt"
	"gez/internal/custom"
	"gez/internal/git"
	"gez/internal/workspace"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ── Tabs ──────────────────────────────────────────────────────────────────────

type tab int

const (
	tabProjects  tab = iota // [1] Projects
	tabGit                  // [2] Git
	tabCommands             // [3] Commands
	tabLog                  // [4] Log
)

// ── Git sub-panels (Tab 2) ────────────────────────────────────────────────────

type panel int

const (
	panelFiles panel = iota
	panelDiff
	panelGitLog
)

// ── Data types ────────────────────────────────────────────────────────────────

type fileEntry struct {
	xy   string
	path string
}

func (f fileEntry) staged() bool   { return f.xy[0] != ' ' && f.xy[0] != '?' }
func (f fileEntry) unstaged() bool { return f.xy[1] != ' ' }

type projEntry struct {
	Name    string
	Path    string
	Branch  string
	Ahead   string
	Behind  string
	Changed int
	Valid   bool
}

// ── Messages ──────────────────────────────────────────────────────────────────

type refreshMsg struct{}
type diffLoadedMsg string
type flashMsg string
type projLoadedMsg []projEntry
type fullLogLoadedMsg string

// ── Model ─────────────────────────────────────────────────────────────────────

type Model struct {
	width, height int
	activeTab     tab

	// ── Shared status bar ──
	branch     string
	ahead      string
	behind     string
	stashCount int
	flowLabel  string
	flash      string

	// ── Tab 1: Projects ──
	projEntries []projEntry
	projCursor  int
	projReady   bool

	// ── Tab 2: Git ──
	gitPanel   panel
	files      []fileEntry
	fileCursor int
	diffVP     viewport.Model
	diffReady  bool
	gitLog     []string
	gitLogOff  int

	// ── Tab 3: Commands ──
	customCfg  custom.Config
	cmdItems   []custom.FlatItem // flat list with headers
	cmdCursor  int               // index into cmdItems (skips headers)
	cmdOutVP   viewport.Model

	// ── Tab 4: Full Log ──
	fullLogVP viewport.Model

	exe string
}

// New creates an initialised Model.
func New() Model {
	diffVP := viewport.New(0, 0)
	fullLogVP := viewport.New(0, 0)
	cmdOutVP := viewport.New(0, 0)
	exe, _ := os.Executable()
	return Model{
		diffVP:    diffVP,
		fullLogVP: fullLogVP,
		cmdOutVP:  cmdOutVP,
		activeTab: tabGit, // start on Git tab (most common use)
		customCfg: custom.Load(),
		exe:       exe,
	}
}

// ── Init ──────────────────────────────────────────────────────────────────────

func (m Model) Init() tea.Cmd {
	return tea.Batch(refresh(), m.loadCmdItems())
}

func refresh() tea.Cmd {
	return func() tea.Msg { return refreshMsg{} }
}

// ── Update ────────────────────────────────────────────────────────────────────

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.resizePanels()
		return m, nil

	case refreshMsg:
		m.loadGitState()
		return m, tea.Batch(m.loadDiffCmd(), m.loadFullLogCmd())

	case diffLoadedMsg:
		m.diffVP.SetContent(string(msg))
		m.diffReady = true
		return m, nil

	case fullLogLoadedMsg:
		m.fullLogVP.SetContent(string(msg))
		return m, nil

	case flashMsg:
		m.flash = string(msg)
		return m, nil

	case projLoadedMsg:
		m.projEntries = []projEntry(msg)
		m.projReady = true
		if m.projCursor >= len(m.projEntries) {
			m.projCursor = 0
		}
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	// Forward scroll events to active viewport
	switch m.activeTab {
	case tabGit:
		if m.gitPanel == panelDiff {
			var cmd tea.Cmd
			m.diffVP, cmd = m.diffVP.Update(msg)
			return m, cmd
		}
	case tabCommands:
		var cmd tea.Cmd
		m.cmdOutVP, cmd = m.cmdOutVP.Update(msg)
		return m, cmd
	case tabLog:
		var cmd tea.Cmd
		m.fullLogVP, cmd = m.fullLogVP.Update(msg)
		return m, cmd
	}
	return m, nil
}

// ── Key handling ──────────────────────────────────────────────────────────────

func (m Model) handleKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	key := msg.String()

	// ── Global ────────────────────────────────────────────────────────────────
	switch key {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "r":
		m.flash = "새로고침 중…"
		return m, tea.Batch(refresh(), loadProjects())
	case "tab":
		next := (m.activeTab + 1) % 4
		return m.switchTab(next)
	case "1":
		return m.switchTab(tabProjects)
	case "2":
		return m.switchTab(tabGit)
	case "3":
		return m.switchTab(tabCommands)
	case "4":
		return m.switchTab(tabLog)
	}

	// ── Tab-specific ──────────────────────────────────────────────────────────
	switch m.activeTab {
	case tabProjects:
		return m.handleProjectsKey(key)
	case tabGit:
		return m.handleGitKey(key)
	case tabCommands:
		return m.handleCommandsKey(key)
	case tabLog:
		return m.handleLogKey(key)
	}
	return m, nil
}

func (m Model) switchTab(t tab) (Model, tea.Cmd) {
	m.activeTab = t
	m.flash = ""
	switch t {
	case tabProjects:
		if !m.projReady {
			return m, loadProjects()
		}
		return m, loadProjects() // always refresh
	case tabGit:
		return m, refresh()
	case tabCommands:
		return m, m.loadCmdItems()
	case tabLog:
		return m, m.loadFullLogCmd()
	}
	return m, nil
}

// ── Projects tab keys ─────────────────────────────────────────────────────────

func (m Model) handleProjectsKey(key string) (Model, tea.Cmd) {
	switch key {
	case "up", "k":
		if m.projCursor > 0 {
			m.projCursor--
		}
	case "down", "j":
		if m.projCursor < len(m.projEntries)-1 {
			m.projCursor++
		}
	case "enter":
		if m.projCursor < len(m.projEntries) {
			e := m.projEntries[m.projCursor]
			if e.Valid {
				if err := os.Chdir(e.Path); err == nil {
					m.flash = fmt.Sprintf("✔ 프로젝트 전환: %s", e.Name)
					// reload cmd items for new project
					m.customCfg = custom.Load()
					return m, tea.Batch(refresh(), m.loadCmdItems())
				}
			}
		}
	case "2", "g":
		if m.projCursor < len(m.projEntries) {
			e := m.projEntries[m.projCursor]
			if e.Valid {
				_ = os.Chdir(e.Path)
			}
		}
		return m.switchTab(tabGit)
	case "3", "c":
		if m.projCursor < len(m.projEntries) {
			e := m.projEntries[m.projCursor]
			if e.Valid {
				_ = os.Chdir(e.Path)
			}
		}
		return m.switchTab(tabCommands)
	}
	return m, nil
}

// ── Git tab keys ──────────────────────────────────────────────────────────────

func (m Model) handleGitKey(key string) (Model, tea.Cmd) {
	switch key {
	// Panel navigation
	case "tab":
		m.gitPanel = (m.gitPanel + 1) % 3
		return m, nil

	// Sub-commands (ExecProcess)
	case "c":
		return m, m.execGez("commit")
	case "p":
		return m, m.execGez("push")
	case "P":
		return m, m.execGez("pull")
	case "b":
		return m, m.execGez("branch", "switch")
	case "s":
		return m, m.execGez("stash")
	case "m":
		return m, m.execGez("merge")
	case "R":
		return m, m.execGez("rebase")
	case "l":
		return m, m.execGez("log", "-i")
	case "h":
		if m.gitPanel == panelFiles && len(m.files) > 0 {
			return m, m.execGit("add", "-p", m.files[m.fileCursor].path)
		}
	case "d":
		if m.gitPanel == panelFiles && len(m.files) > 0 {
			return m, m.execGit("diff", "--color=always", "--", m.files[m.fileCursor].path)
		}

	// File navigation
	case "up", "k":
		if m.gitPanel == panelFiles && m.fileCursor > 0 {
			m.fileCursor--
			return m, m.loadDiffCmd()
		}
		if m.gitPanel == panelGitLog && m.gitLogOff > 0 {
			m.gitLogOff--
		}
	case "down", "j":
		if m.gitPanel == panelFiles && m.fileCursor < len(m.files)-1 {
			m.fileCursor++
			return m, m.loadDiffCmd()
		}
		if m.gitPanel == panelGitLog {
			max := len(m.gitLog) - gitLogVisible(m.height)
			if max < 0 {
				max = 0
			}
			if m.gitLogOff < max {
				m.gitLogOff++
			}
		}

	// Stage / unstage
	case " ":
		if m.gitPanel == panelFiles && len(m.files) > 0 {
			f := m.files[m.fileCursor]
			if f.staged() {
				if _, err := git.Run("restore", "--staged", f.path); err != nil {
					m.flash = "unstage 실패: " + err.Error()
				} else {
					m.flash = fmt.Sprintf("✔ unstaged: %s", f.path)
				}
			} else {
				if _, err := git.Run("add", f.path); err != nil {
					m.flash = "stage 실패: " + err.Error()
				} else {
					m.flash = fmt.Sprintf("✔ staged: %s", f.path)
				}
			}
			return m, refresh()
		}
	case "a":
		if _, err := git.Run("add", "-A"); err != nil {
			m.flash = "stage all 실패: " + err.Error()
		} else {
			m.flash = "✔ 전체 staged"
		}
		return m, refresh()
	case "u":
		if _, err := git.Run("restore", "--staged", "."); err != nil {
			m.flash = "unstage all 실패"
		} else {
			m.flash = "✔ 전체 unstaged"
		}
		return m, refresh()

	// Diff scroll
	case "pgup", "ctrl+u":
		m.diffVP.HalfViewUp()
	case "pgdown", "ctrl+d":
		m.diffVP.HalfViewDown()
	}
	return m, nil
}

// ── Commands tab keys ─────────────────────────────────────────────────────────

func (m Model) handleCommandsKey(key string) (Model, tea.Cmd) {
	switch key {
	case "up", "k":
		m.moveCmdCursor(-1)
	case "down", "j":
		m.moveCmdCursor(1)
	case "enter":
		if item := m.selectedCmdItem(); item != nil {
			cmdStr := item.Cmd.Cmd()
			if cmdStr == "" {
				m.flash = "이 플랫폼에서 지원하지 않는 명령어입니다"
				return m, nil
			}
			return m, m.execShell(cmdStr)
		}
	case "pgup", "ctrl+u":
		m.cmdOutVP.HalfViewUp()
	case "pgdown", "ctrl+d":
		m.cmdOutVP.HalfViewDown()
	}
	return m, nil
}

func (m *Model) moveCmdCursor(delta int) {
	if len(m.cmdItems) == 0 {
		return
	}
	next := m.cmdCursor + delta
	// Skip headers
	for next >= 0 && next < len(m.cmdItems) && m.cmdItems[next].IsHeader {
		next += delta
	}
	if next >= 0 && next < len(m.cmdItems) {
		m.cmdCursor = next
	}
}

func (m *Model) selectedCmdItem() *custom.FlatItem {
	if m.cmdCursor < len(m.cmdItems) {
		item := m.cmdItems[m.cmdCursor]
		if !item.IsHeader {
			return &item
		}
	}
	return nil
}

// ── Log tab keys ──────────────────────────────────────────────────────────────

func (m Model) handleLogKey(key string) (Model, tea.Cmd) {
	switch key {
	case "pgup", "ctrl+u", "up", "k":
		m.fullLogVP.HalfViewUp()
	case "pgdown", "ctrl+d", "down", "j":
		m.fullLogVP.HalfViewDown()
	case "enter", "i":
		return m, m.execGez("log", "-i")
	}
	return m, nil
}

// ── Subprocess launchers ──────────────────────────────────────────────────────

func (m *Model) execGez(args ...string) tea.Cmd {
	exe := m.exe
	if exe == "" {
		exe = "gez"
	}
	c := exec.Command(exe, args...)
	c.Stdin = os.Stdin
	return tea.ExecProcess(c, func(err error) tea.Msg { return refreshMsg{} })
}

func (m *Model) execGit(args ...string) tea.Cmd {
	c := exec.Command("git", args...)
	c.Stdin = os.Stdin
	return tea.ExecProcess(c, func(err error) tea.Msg { return refreshMsg{} })
}

func (m *Model) execShell(cmdStr string) tea.Cmd {
	cwd, _ := os.Getwd()
	var c *exec.Cmd
	if runtime.GOOS == "windows" {
		c = exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", cmdStr)
	} else {
		c = exec.Command("bash", "-c", cmdStr)
	}
	c.Dir = cwd
	c.Stdin = os.Stdin
	return tea.ExecProcess(c, func(err error) tea.Msg {
		if err != nil {
			return flashMsg("⚠ 명령 종료: " + err.Error())
		}
		return flashMsg("✔ 완료")
	})
}

// ── Data loading ──────────────────────────────────────────────────────────────

func (m *Model) loadGitState() {
	m.branch = git.CurrentBranch()
	m.ahead, m.behind = git.AheadBehind()
	m.stashCount = len(git.StashList())
	m.flowLabel = loadFlowLabel()
	m.flash = ""

	// Files
	lines := git.StatusShort()
	m.files = nil
	for _, l := range lines {
		if len(l) < 3 {
			continue
		}
		m.files = append(m.files, fileEntry{
			xy:   l[:2],
			path: strings.TrimSpace(l[3:]),
		})
	}
	if m.fileCursor >= len(m.files) {
		if len(m.files) == 0 {
			m.fileCursor = 0
		} else {
			m.fileCursor = len(m.files) - 1
		}
	}

	// Git log
	m.gitLog = git.RecentCommits(80)
}

func (m Model) loadDiffCmd() tea.Cmd {
	if len(m.files) == 0 {
		return func() tea.Msg { return diffLoadedMsg("변경된 파일 없음") }
	}
	f := m.files[m.fileCursor]
	return func() tea.Msg {
		var out string
		if f.staged() {
			out, _ = git.Run("diff", "--cached", "--", f.path)
		} else {
			out, _ = git.Run("diff", "--", f.path)
		}
		if out == "" {
			out2, _ := git.Run("show", ":"+f.path)
			if out2 != "" {
				out = "(새 파일)\n" + out2
			} else {
				out = "(diff 없음)"
			}
		}
		return diffLoadedMsg(colorizeDiff(out))
	}
}

func (m Model) loadFullLogCmd() tea.Cmd {
	return func() tea.Msg {
		out, _ := git.Run("log", "--graph", "--oneline", "--decorate", "--color=never", "-100")
		return fullLogLoadedMsg(colorizeLog(out))
	}
}

func (m Model) loadCmdItems() tea.Cmd {
	return func() tea.Msg {
		// no-op; just trigger cmd items refresh on next render
		return refreshMsg{}
	}
}

func loadProjects() tea.Cmd {
	return func() tea.Msg {
		ws, err := workspace.Load()
		if err != nil || len(ws.Projects) == 0 {
			return projLoadedMsg(nil)
		}
		entries := make([]projEntry, len(ws.Projects))
		for i, p := range ws.Projects {
			e := projEntry{Name: p.Name, Path: p.Path}
			if git.IsRepoInDir(p.Path) {
				e.Valid = true
				e.Branch = git.CurrentBranchInDir(p.Path)
				e.Ahead, e.Behind = git.AheadBehindInDir(p.Path)
				for _, l := range git.StatusShortInDir(p.Path) {
					if len(l) >= 3 {
						e.Changed++
					}
				}
			}
			entries[i] = e
		}
		return projLoadedMsg(entries)
	}
}

// currentProjectName derives the project name from cwd vs workspace.
func currentProjectName() string {
	cwd, _ := os.Getwd()
	ws, err := workspace.Load()
	if err == nil {
		for _, p := range ws.Projects {
			if filepath.Clean(p.Path) == filepath.Clean(cwd) {
				return p.Name
			}
		}
	}
	return filepath.Base(cwd)
}

func loadFlowLabel() string {
	out, err := git.Run("config", "--local", "--get", "gez.flow.strategy")
	if err != nil || strings.TrimSpace(out) == "" {
		return ""
	}
	switch strings.TrimSpace(out) {
	case "gitflow":
		return "GitFlow"
	case "githubflow":
		return "GitHub Flow"
	case "trunk":
		return "Trunk"
	}
	return ""
}

// ── Layout ────────────────────────────────────────────────────────────────────

func (m *Model) resizePanels() {
	mainH := m.contentHeight()
	leftW := m.width * 30 / 100
	rightW := m.width - leftW - 1
	if rightW < 10 {
		rightW = 10
	}

	// Git tab diff viewport
	m.diffVP.Width = rightW - 4
	m.diffVP.Height = mainH - 10 // leave room for log strip

	// Commands output viewport
	m.cmdOutVP.Width = rightW - 4
	m.cmdOutVP.Height = mainH - 2

	// Full log viewport
	m.fullLogVP.Width = m.width - 4
	m.fullLogVP.Height = mainH - 2
}

// contentHeight returns usable height below status+tab bars and above help bar.
func (m Model) contentHeight() int {
	h := m.height - 4 // status bar(1) + tab bar(1) + help bar(1) + padding(1)
	if h < 5 {
		h = 5
	}
	return h
}

func gitLogVisible(h int) int {
	v := 6 - 2
	if v < 1 {
		v = 1
	}
	return v
}

// ── View ──────────────────────────────────────────────────────────────────────

var (
	styleBorderActive   = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("39"))
	styleBorderInactive = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("240"))
	styleHeader         = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("39"))
	styleStatusBar      = lipgloss.NewStyle().Background(lipgloss.Color("237")).Foreground(lipgloss.Color("255"))
	styleTabBar         = lipgloss.NewStyle().Background(lipgloss.Color("234")).Foreground(lipgloss.Color("245"))
	styleTabActive      = lipgloss.NewStyle().Background(lipgloss.Color("39")).Foreground(lipgloss.Color("0")).Bold(true).Padding(0, 1)
	styleTabInactive    = lipgloss.NewStyle().Background(lipgloss.Color("237")).Foreground(lipgloss.Color("252")).Padding(0, 1)
	styleFlash          = lipgloss.NewStyle().Foreground(lipgloss.Color("220"))
	styleStagedXY       = lipgloss.NewStyle().Foreground(lipgloss.Color("82"))
	styleUnstagedXY     = lipgloss.NewStyle().Foreground(lipgloss.Color("208"))
	styleUntrackedXY    = lipgloss.NewStyle().Foreground(lipgloss.Color("117"))
	styleSelected       = lipgloss.NewStyle().Background(lipgloss.Color("237")).Bold(true)
	styleDim            = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	styleLogHash        = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	styleLogMsg         = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	styleGreen          = lipgloss.NewStyle().Foreground(lipgloss.Color("82"))
	styleRed            = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	styleYellow         = lipgloss.NewStyle().Foreground(lipgloss.Color("220"))
	styleCyan           = lipgloss.NewStyle().Foreground(lipgloss.Color("39"))
	styleGroupHeader    = lipgloss.NewStyle().Foreground(lipgloss.Color("75")).Bold(true)
)

func (m Model) View() string {
	if m.width == 0 {
		return "로딩 중…"
	}

	statusBar := m.renderStatusBar()
	tabBar := m.renderTabBar()
	var content string
	switch m.activeTab {
	case tabProjects:
		content = m.renderProjectsTab()
	case tabGit:
		content = m.renderGitTab()
	case tabCommands:
		content = m.renderCommandsTab()
	case tabLog:
		content = m.renderLogTab()
	}
	helpBar := m.renderHelpBar()

	return lipgloss.JoinVertical(lipgloss.Left, statusBar, tabBar, content, helpBar)
}

// ── Status bar ────────────────────────────────────────────────────────────────

func (m Model) renderStatusBar() string {
	branch := styleHeader.Render("⎇  " + m.branch)
	sync := ""
	if m.ahead != "" && m.ahead != "0" {
		sync += styleGreen.Render(fmt.Sprintf(" ↑%s", m.ahead))
	}
	if m.behind != "" && m.behind != "0" {
		sync += styleRed.Render(fmt.Sprintf(" ↓%s", m.behind))
	}
	flash := ""
	if m.flash != "" {
		flash = "  " + styleFlash.Render(m.flash)
	}
	left := "  " + branch + sync + flash

	right := ""
	if m.stashCount > 0 {
		right += styleYellow.Render(fmt.Sprintf("stash:%d  ", m.stashCount))
	}
	if m.flowLabel != "" {
		right += styleCyan.Render(m.flowLabel + "  ")
	}
	right += styleDim.Render("gez TUI  ")

	gap := m.width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 0 {
		gap = 0
	}
	return styleStatusBar.Width(m.width).Render(left + strings.Repeat(" ", gap) + right)
}

// ── Tab bar ───────────────────────────────────────────────────────────────────

func (m Model) renderTabBar() string {
	tabs := []struct {
		key   string
		label string
		t     tab
	}{
		{"1", "Projects", tabProjects},
		{"2", "Git", tabGit},
		{"3", "Commands", tabCommands},
		{"4", "Log", tabLog},
	}
	var parts []string
	for _, t := range tabs {
		label := fmt.Sprintf("[%s] %s", t.key, t.label)
		if m.activeTab == t.t {
			parts = append(parts, styleTabActive.Render(label))
		} else {
			parts = append(parts, styleTabInactive.Render(label))
		}
	}
	bar := " " + strings.Join(parts, " ")
	pad := m.width - lipgloss.Width(bar)
	if pad > 0 {
		bar += styleTabBar.Render(strings.Repeat(" ", pad))
	}
	return bar
}

// ── Help bar ──────────────────────────────────────────────────────────────────

func (m Model) renderHelpBar() string {
	switch m.activeTab {
	case tabProjects:
		return styleDim.Render("  ↑↓:이동  enter:프로젝트 전환  g:→Git  c:→Commands  r:새로고침  q:종료")
	case tabGit:
		return styleDim.Render("  space:stage  a:all  u:unstage  h:hunk  d:diff  c:커밋  p:push  P:pull  b:브랜치  s:stash  m:머지  l:로그  tab:패널  r:새로고침  q:종료")
	case tabCommands:
		return styleDim.Render("  ↑↓:이동  enter:실행  ↕PgUp/PgDn:출력 스크롤  r:새로고침  q:종료")
	case tabLog:
		return styleDim.Render("  ↑↓/PgUp/PgDn:스크롤  enter/i:대화형 로그  r:새로고침  q:종료")
	}
	return ""
}

// ── Tab 1: Projects ───────────────────────────────────────────────────────────

func (m Model) renderProjectsTab() string {
	h := m.contentHeight()
	leftW := m.width * 35 / 100
	rightW := m.width - leftW - 1

	// Left: project list
	listContent := m.renderProjectList(h - 2)
	leftBorder := styleBorderActive
	leftPanel := leftBorder.Width(leftW).Height(h).Render(listContent)

	// Right: project detail
	detailContent := m.renderProjectDetail(h - 2)
	rightPanel := styleBorderInactive.Width(rightW).Height(h).Render(detailContent)

	return lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, rightPanel)
}

func (m Model) renderProjectList(maxLines int) string {
	title := styleHeader.Render("워크스페이스 프로젝트")
	lines := []string{title, ""}

	if !m.projReady {
		lines = append(lines, styleDim.Render("  로딩 중…"))
	} else if len(m.projEntries) == 0 {
		lines = append(lines, styleDim.Render("  (등록된 프로젝트 없음)"))
		lines = append(lines, "")
		lines = append(lines, styleDim.Render("  gez ws add 로 등록하세요"))
	} else {
		for i, e := range m.projEntries {
			indicator := "  "
			if i == m.projCursor {
				indicator = styleSelected.Render("▶ ")
			}
			name := e.Name
			if i == m.projCursor {
				name = styleSelected.Render(name)
			} else {
				name = styleHeader.Render(name)
			}

			status := ""
			if !e.Valid {
				status = styleRed.Render(" ⚠")
			} else if e.Changed > 0 {
				status = styleYellow.Render(fmt.Sprintf(" ●%d", e.Changed))
			}
			lines = append(lines, fmt.Sprintf("%s%s%s", indicator, name, status))
		}
	}

	for len(lines) < maxLines {
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}

func (m Model) renderProjectDetail(maxLines int) string {
	if len(m.projEntries) == 0 || m.projCursor >= len(m.projEntries) {
		return styleDim.Render("  프로젝트를 선택하세요")
	}
	e := m.projEntries[m.projCursor]
	lines := []string{styleHeader.Render(e.Name), ""}

	if !e.Valid {
		lines = append(lines, styleRed.Render("  ⚠ 경로 없음 또는 git 저장소 아님"))
		lines = append(lines, styleDim.Render("  "+e.Path))
	} else {
		// Branch + sync
		branchStr := styleCyan.Render("⎇  " + e.Branch)
		sync := ""
		if e.Ahead != "" && e.Ahead != "0" {
			sync += styleGreen.Render(" ↑" + e.Ahead)
		}
		if e.Behind != "" && e.Behind != "0" {
			sync += styleRed.Render(" ↓" + e.Behind)
		}
		lines = append(lines, "  "+branchStr+sync)
		lines = append(lines, "")

		// Path
		lines = append(lines, styleDim.Render("  "+e.Path))
		lines = append(lines, "")

		// Changes
		if e.Changed == 0 {
			lines = append(lines, styleGreen.Render("  ✔ 워킹 트리 깨끗"))
		} else {
			lines = append(lines, styleYellow.Render(fmt.Sprintf("  %d 파일 변경됨", e.Changed)))
		}
		lines = append(lines, "")
		lines = append(lines, styleDim.Render("  enter:전환  g:Git탭  c:Commands탭"))
	}

	for len(lines) < maxLines {
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}

// ── Tab 2: Git ────────────────────────────────────────────────────────────────

func (m Model) renderGitTab() string {
	logH := 6
	h := m.contentHeight()
	mainH := h - logH
	if mainH < 4 {
		mainH = 4
	}
	leftW := m.width * 30 / 100
	rightW := m.width - leftW - 1

	// Files panel
	filesBorder := styleBorderInactive
	if m.gitPanel == panelFiles {
		filesBorder = styleBorderActive
	}
	filesPanel := filesBorder.Width(leftW).Height(mainH).Render(m.renderFiles(mainH - 2))

	// Diff panel
	diffBorder := styleBorderInactive
	if m.gitPanel == panelDiff {
		diffBorder = styleBorderActive
	}
	m.diffVP.Width = rightW - 4
	m.diffVP.Height = mainH - 2
	diffPanel := diffBorder.Width(rightW).Height(mainH).Render(m.diffVP.View())

	mainRow := lipgloss.JoinHorizontal(lipgloss.Top, filesPanel, diffPanel)

	// Git log strip
	logBorder := styleBorderInactive
	if m.gitPanel == panelGitLog {
		logBorder = styleBorderActive
	}
	logPanel := logBorder.Width(m.width - 2).Height(logH).Render(m.renderGitLog(logH - 2))

	return lipgloss.JoinVertical(lipgloss.Left, mainRow, logPanel)
}

func (m Model) renderFiles(maxLines int) string {
	title := styleHeader.Render("변경 파일")
	lines := []string{title, ""}
	if len(m.files) == 0 {
		lines = append(lines, styleDim.Render("  (변경 없음)"))
	}
	for i, f := range m.files {
		xy := xyStyle(f.xy).Render(f.xy)
		path := f.path
		if i == m.fileCursor && m.gitPanel == panelFiles {
			path = styleSelected.Render("▶ " + path)
		} else {
			path = "  " + path
		}
		lines = append(lines, fmt.Sprintf(" %s %s", xy, path))
	}
	for len(lines) < maxLines {
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}

func (m Model) renderGitLog(maxLines int) string {
	title := styleHeader.Render("최근 커밋")
	lines := []string{title, ""}
	end := m.gitLogOff + maxLines - 2
	if end > len(m.gitLog) {
		end = len(m.gitLog)
	}
	if m.gitLogOff < len(m.gitLog) {
		for _, l := range m.gitLog[m.gitLogOff:end] {
			parts := strings.SplitN(l, " ", 2)
			if len(parts) == 2 {
				lines = append(lines, styleLogHash.Render(parts[0])+"  "+styleLogMsg.Render(parts[1]))
			} else {
				lines = append(lines, l)
			}
		}
	}
	return strings.Join(lines, "\n")
}

// ── Tab 3: Commands ───────────────────────────────────────────────────────────

func (m Model) renderCommandsTab() string {
	h := m.contentHeight()
	leftW := m.width * 35 / 100
	rightW := m.width - leftW - 1

	// Re-build flat items for the current project
	projName := currentProjectName()
	projCfg := m.customCfg.ForProject(projName)
	items := custom.FlatList(projCfg.Commands)

	// Left: command list
	listContent := renderCmdList(items, m.cmdCursor, projName, h-2)
	leftBorder := styleBorderActive
	leftPanel := leftBorder.Width(leftW).Height(h).Render(listContent)

	// Right: instructions + output viewport
	rightContent := m.renderCmdRight(items, h-2)
	rightPanel := styleBorderInactive.Width(rightW).Height(h).Render(rightContent)

	return lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, rightPanel)
}

func renderCmdList(items []custom.FlatItem, cursor int, projName string, maxLines int) string {
	title := styleHeader.Render("Commands: " + projName)
	lines := []string{title, ""}

	if len(items) == 0 {
		lines = append(lines, styleDim.Render("  (등록된 명령어 없음)"))
		lines = append(lines, "")
		lines = append(lines, styleDim.Render("  gez custom add 로 추가"))
	}

	for i, item := range items {
		if item.IsHeader {
			lines = append(lines, styleGroupHeader.Render("  ── "+item.Header+" ──"))
			continue
		}
		prefix := "   "
		name := item.Cmd.Name
		if i == cursor {
			prefix = styleSelected.Render(" ▶ ")
			name = styleSelected.Render(fmt.Sprintf("%-18s", name))
		} else {
			name = styleCyan.Render(fmt.Sprintf("%-18s", name))
		}
		desc := styleDim.Render(item.Cmd.Description)
		lines = append(lines, prefix+name+" "+desc)
	}

	for len(lines) < maxLines {
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}

func (m Model) renderCmdRight(items []custom.FlatItem, maxLines int) string {
	lines := []string{styleHeader.Render("명령어 실행"), ""}

	// Show selected command details
	if m.cmdCursor < len(items) {
		item := items[m.cmdCursor]
		if !item.IsHeader {
			cmdStr := item.Cmd.Cmd()
			lines = append(lines, styleCyan.Render("  "+item.Cmd.Name))
			lines = append(lines, styleDim.Render("  "+item.Cmd.Description))
			lines = append(lines, "")
			if cmdStr != "" {
				lines = append(lines, styleDim.Render("  $ "+cmdStr))
			} else {
				lines = append(lines, styleRed.Render("  (이 플랫폼에서 지원하지 않음)"))
			}
			lines = append(lines, "")
			lines = append(lines, styleDim.Render("  enter 키로 실행"))
		}
	}

	lines = append(lines, strings.Repeat("─", 30))
	lines = append(lines, styleDim.Render("커스텀 명령어 관리:"))
	lines = append(lines, styleDim.Render("  gez custom add   — 명령어 추가"))
	lines = append(lines, styleDim.Render("  gez custom rm    — 명령어 삭제"))
	lines = append(lines, styleDim.Render("  gez custom ls    — 전체 목록"))

	for len(lines) < maxLines {
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}

// ── Tab 4: Full Log ───────────────────────────────────────────────────────────

func (m Model) renderLogTab() string {
	h := m.contentHeight()
	m.fullLogVP.Width = m.width - 4
	m.fullLogVP.Height = h - 2
	title := styleHeader.Render("커밋 로그 (최근 100개)")
	hint := styleDim.Render("  enter / i : 대화형 선택 → show·cherry-pick·reset")
	content := lipgloss.JoinVertical(lipgloss.Left, title, "", m.fullLogVP.View(), "", hint)
	return styleBorderActive.Width(m.width-2).Height(h).Render(content)
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func xyStyle(xy string) lipgloss.Style {
	if len(xy) < 2 {
		return styleDim
	}
	if xy[0] != ' ' && xy[0] != '?' {
		return styleStagedXY
	}
	if xy == "??" {
		return styleUntrackedXY
	}
	return styleUnstagedXY
}

func colorizeDiff(s string) string {
	addStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("82"))
	delStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	hunkStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("39"))
	hdrStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Bold(true)

	var sb strings.Builder
	for _, line := range strings.Split(s, "\n") {
		switch {
		case strings.HasPrefix(line, "+++") || strings.HasPrefix(line, "---"):
			sb.WriteString(hdrStyle.Render(line))
		case strings.HasPrefix(line, "@@"):
			sb.WriteString(hunkStyle.Render(line))
		case strings.HasPrefix(line, "+"):
			sb.WriteString(addStyle.Render(line))
		case strings.HasPrefix(line, "-"):
			sb.WriteString(delStyle.Render(line))
		default:
			sb.WriteString(line)
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

func colorizeLog(s string) string {
	hashStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	branchStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("39"))
	tagStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("220"))

	var sb strings.Builder
	for _, line := range strings.Split(s, "\n") {
		// color (HEAD -> branch, tag: ...) references
		processed := line
		if idx := strings.Index(line, " "); idx > 0 {
			hash := line[:idx]
			rest := line[idx:]
			rest = strings.ReplaceAll(rest, "(", branchStyle.Render("("))
			rest = strings.ReplaceAll(rest, "tag:", tagStyle.Render("tag:"))
			processed = hashStyle.Render(hash) + rest
		}
		sb.WriteString(processed + "\n")
	}
	return sb.String()
}
