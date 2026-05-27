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
	tabProjects tab = iota // [1]
	tabGit                 // [2]
	tabCommands            // [3]
	tabLog                 // [4]
)

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
	Name, Path, Branch, Ahead, Behind string
	Changed                           int
	Valid                             bool
}

// paletteItem represents one gez command in the command palette.
type paletteItem struct {
	name string
	args []string
	desc string
}

// ── Messages ──────────────────────────────────────────────────────────────────

type refreshMsg struct{}
type diffLoadedMsg string
type flashMsg string
type projLoadedMsg []projEntry
type fullLogLoadedMsg string

// ── Click zone ────────────────────────────────────────────────────────────────

// btnZone tracks one clickable button in the button bar.
type btnZone struct {
	x1, x2 int    // column range [x1, x2)
	key    string // key to fire
}

// tabBtnDef defines one button for a tab's button bar.
type tabBtnDef struct {
	label string
	key   string
	style lipgloss.Style
	sep   bool // if true, renders "│" separator, key is ignored
}

// ── Model ─────────────────────────────────────────────────────────────────────

type Model struct {
	width, height int
	activeTab     tab

	// Status bar (shared)
	branch, ahead, behind string
	stashCount            int
	flowLabel             string
	flash                 string

	// Tab 1: Projects
	projEntries []projEntry
	projCursor  int
	projReady   bool

	// Tab 2: Git
	gitPanel   panel
	files      []fileEntry
	fileCursor int
	diffVP     viewport.Model
	gitLog     []string
	gitLogOff  int

	// Tab 3: Commands
	customCfg custom.Config
	cmdItems  []custom.FlatItem // populated on tab switch
	cmdCursor int

	// Tab 4: Log
	fullLogVP viewport.Model

	// Command palette (':' key — access all gez CLI commands)
	paletteOpen   bool
	paletteInput  string
	paletteItems  []paletteItem
	paletteCursor int

	// Mouse click support
	btnZones      []btnZone // button bar zones (rebuilt on tab switch / resize)
	lastClickY    int       // for double-click detection
	lastClickItem int       // item index of last click

	exe string
}

// New creates an initialised Model.
func New() Model {
	exe, _ := os.Executable()
	startTab := tabGit
	if !git.IsRepo() {
		startTab = tabProjects
	}
	m := Model{
		diffVP:    viewport.New(0, 0),
		fullLogVP: viewport.New(0, 0),
		activeTab: startTab,
		customCfg: custom.Load(),
		exe:       exe,
		lastClickY:    -1,
		lastClickItem: -1,
	}
	m.paletteItems = allGezCommands
	return m
}

// ── Init ──────────────────────────────────────────────────────────────────────

func (m Model) Init() tea.Cmd {
	if m.activeTab == tabProjects {
		return loadProjects()
	}
	return refresh()
}

func refresh() tea.Cmd { return func() tea.Msg { return refreshMsg{} } }

// ── Update ────────────────────────────────────────────────────────────────────

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.resizePanels()
		m.rebuildBtnZones()
		return m, nil

	case tea.MouseMsg:
		if msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft {
			return m.handleClick(msg.X, msg.Y)
		}
		// Scroll wheel in content area
		if msg.Button == tea.MouseButtonWheelUp {
			return m.handleMouseScroll(-3)
		}
		if msg.Button == tea.MouseButtonWheelDown {
			return m.handleMouseScroll(3)
		}

	case refreshMsg:
		m.loadGitState()
		if m.activeTab == tabCommands {
			m.rebuildCmdItems()
		}
		return m, tea.Batch(m.loadDiffCmd(), m.loadFullLogCmd())

	case diffLoadedMsg:
		m.diffVP.SetContent(string(msg))
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

	// Forward scroll to active viewport
	switch {
	case m.paletteOpen:
		// no scroll forwarding while palette is open
	case m.activeTab == tabGit && m.gitPanel == panelDiff:
		var cmd tea.Cmd
		m.diffVP, cmd = m.diffVP.Update(msg)
		return m, cmd
	case m.activeTab == tabLog:
		var cmd tea.Cmd
		m.fullLogVP, cmd = m.fullLogVP.Update(msg)
		return m, cmd
	}
	return m, nil
}

// ── Key dispatch ──────────────────────────────────────────────────────────────

func (m Model) handleKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	key := msg.String()

	// ── Command palette takes priority ────────────────────────────────────────
	if m.paletteOpen {
		return m.handlePaletteKey(key)
	}

	// ── Open palette ──────────────────────────────────────────────────────────
	if key == ":" {
		m.paletteOpen = true
		m.paletteInput = ""
		m.paletteItems = allGezCommands
		m.paletteCursor = 0
		return m, nil
	}

	// ── Global (always available) ─────────────────────────────────────────────
	switch key {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "r":
		m.flash = "새로고침 중…"
		return m, tea.Batch(refresh(), loadProjects())
	// Direct tab switch by number
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

// ── Tab switching ─────────────────────────────────────────────────────────────

func (m Model) switchTab(t tab) (Model, tea.Cmd) {
	prev := m.activeTab
	m.activeTab = t
	m.flash = ""
	_ = prev

	m.rebuildBtnZones()

	switch t {
	case tabProjects:
		return m, loadProjects()
	case tabGit:
		return m, refresh()
	case tabCommands:
		m.rebuildCmdItems()
		return m, nil
	case tabLog:
		return m, m.loadFullLogCmd()
	}
	return m, nil
}

// ── Command palette ───────────────────────────────────────────────────────────

func (m Model) handlePaletteKey(key string) (Model, tea.Cmd) {
	switch key {
	case "esc", "ctrl+c":
		m.paletteOpen = false
		m.paletteInput = ""
		return m, nil

	case "enter":
		if m.paletteCursor < len(m.paletteItems) {
			item := m.paletteItems[m.paletteCursor]
			m.paletteOpen = false
			m.paletteInput = ""
			return m, m.execGez(item.args...)
		}

	case "up", "k":
		if m.paletteCursor > 0 {
			m.paletteCursor--
		}

	case "down", "j":
		if m.paletteCursor < len(m.paletteItems)-1 {
			m.paletteCursor++
		}

	case "backspace", "ctrl+h":
		if len(m.paletteInput) > 0 {
			m.paletteInput = m.paletteInput[:len(m.paletteInput)-1]
			m.filterPalette()
		}

	default:
		// Printable characters → filter
		if len(key) == 1 && key[0] >= 32 {
			m.paletteInput += key
			m.filterPalette()
		}
	}
	return m, nil
}

func (m *Model) filterPalette() {
	q := strings.ToLower(m.paletteInput)
	m.paletteItems = nil
	for _, item := range allGezCommands {
		if q == "" || strings.Contains(item.name, q) || strings.Contains(item.desc, q) {
			m.paletteItems = append(m.paletteItems, item)
		}
	}
	m.paletteCursor = 0
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
					m.flash = "✔  " + e.Name + " 로 전환"
					m.customCfg = custom.Load()
					return m, refresh()
				}
			}
		}
	case "tab":
		// In Projects tab: tab cycles to next main tab
		return m.switchTab(tabGit)
	case "g":
		if m.projCursor < len(m.projEntries) {
			if m.projEntries[m.projCursor].Valid {
				_ = os.Chdir(m.projEntries[m.projCursor].Path)
			}
		}
		return m.switchTab(tabGit)
	case "c":
		if m.projCursor < len(m.projEntries) {
			if m.projEntries[m.projCursor].Valid {
				_ = os.Chdir(m.projEntries[m.projCursor].Path)
			}
		}
		return m.switchTab(tabCommands)
	case "d":
		// Auto-detect custom commands for selected project
		if m.projCursor < len(m.projEntries) && m.projEntries[m.projCursor].Valid {
			e := m.projEntries[m.projCursor]
			return m, m.execGez("custom", "detect", "--force", e.Path)
		}
	}
	return m, nil
}

// ── Git tab keys ──────────────────────────────────────────────────────────────

func (m Model) handleGitKey(key string) (Model, tea.Cmd) {
	switch key {
	// Tab cycles git sub-panels (files → diff → log → files)
	case "tab":
		m.gitPanel = (m.gitPanel + 1) % 3
		return m, nil

	// ── ExecProcess launchers ──────────────────────────────────────────────
	case "c":
		return m, m.execGez("commit")
	case "p":
		return m, m.execGez("push")
	case "P":
		return m, m.execGez("pull")
	case "f":
		return m, m.execGez("fetch")
	case "S":
		return m, m.execGez("sync")
	case "b":
		return m, m.execGez("branch", "switch")
	case "B":
		return m, m.execGez("branch")
	case "s":
		return m, m.execGez("stash")
	case "m":
		return m, m.execGez("merge")
	case "R":
		return m, m.execGez("rebase")
	case "l":
		return m, m.execGez("log", "-i")
	case "L":
		return m.switchTab(tabLog)
	case "h":
		if m.gitPanel == panelFiles && len(m.files) > 0 {
			return m, m.execGit("add", "-p", m.files[m.fileCursor].path)
		}
	case "d":
		if len(m.files) > 0 {
			return m, m.execGit("diff", "--color=always", "--", m.files[m.fileCursor].path)
		}
	case "D":
		return m, m.execGez("diff")

	// ── Navigation ────────────────────────────────────────────────────────
	case "up", "k":
		switch m.gitPanel {
		case panelFiles:
			if m.fileCursor > 0 {
				m.fileCursor--
				return m, m.loadDiffCmd()
			}
		case panelGitLog:
			if m.gitLogOff > 0 {
				m.gitLogOff--
			}
		case panelDiff:
			m.diffVP.LineUp(3)
		}
	case "down", "j":
		switch m.gitPanel {
		case panelFiles:
			if m.fileCursor < len(m.files)-1 {
				m.fileCursor++
				return m, m.loadDiffCmd()
			}
		case panelGitLog:
			max := len(m.gitLog) - (m.height / 8)
			if max < 0 {
				max = 0
			}
			if m.gitLogOff < max {
				m.gitLogOff++
			}
		case panelDiff:
			m.diffVP.LineDown(3)
		}

	// ── Stage / unstage ───────────────────────────────────────────────────
	case " ":
		if len(m.files) > 0 {
			f := m.files[m.fileCursor]
			if f.staged() {
				if _, err := git.Run("restore", "--staged", f.path); err != nil {
					m.flash = "unstage 실패"
				} else {
					m.flash = "✔ unstaged: " + f.path
				}
			} else {
				if _, err := git.Run("add", f.path); err != nil {
					m.flash = "stage 실패"
				} else {
					m.flash = "✔ staged: " + f.path
				}
			}
			return m, refresh()
		}
	case "a":
		if _, err := git.Run("add", "-A"); err != nil {
			m.flash = "stage all 실패"
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

	// ── Diff scroll ────────────────────────────────────────────────────────
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
	case "tab":
		return m.switchTab(tabLog)

	case "up", "k":
		m.moveCmdCursor(-1)
	case "down", "j":
		m.moveCmdCursor(1)

	case "enter":
		if item := m.currentCmdItem(); item != nil {
			cmdStr := item.Cmd.Cmd()
			if cmdStr == "" {
				m.flash = "⚠ 이 플랫폼에서 지원하지 않는 명령어입니다"
				return m, nil
			}
			return m, m.execShell(cmdStr)
		}

	case "A": // add custom command
		return m, m.execGez("custom", "add")

	case "D": // detect custom commands for this project
		return m, m.execGez("custom", "detect")

	case "X": // remove a custom command
		return m, m.execGez("custom", "remove")

	case "pgup", "ctrl+u":
		m.moveCmdCursor(-5)
	case "pgdown", "ctrl+d":
		m.moveCmdCursor(5)
	}
	return m, nil
}

func (m *Model) moveCmdCursor(delta int) {
	if len(m.cmdItems) == 0 {
		return
	}
	cur := m.cmdCursor + delta
	// clamp
	if cur < 0 {
		cur = 0
	}
	if cur >= len(m.cmdItems) {
		cur = len(m.cmdItems) - 1
	}
	// skip headers
	for cur > 0 && cur < len(m.cmdItems) && m.cmdItems[cur].IsHeader {
		if delta >= 0 {
			cur++
		} else {
			cur--
		}
	}
	if cur >= 0 && cur < len(m.cmdItems) && !m.cmdItems[cur].IsHeader {
		m.cmdCursor = cur
	}
}

func (m *Model) currentCmdItem() *custom.FlatItem {
	if m.cmdCursor < len(m.cmdItems) && !m.cmdItems[m.cmdCursor].IsHeader {
		item := m.cmdItems[m.cmdCursor]
		return &item
	}
	return nil
}

// rebuildCmdItems rebuilds the flat command list for the current project.
func (m *Model) rebuildCmdItems() {
	projName := currentProjectName()
	projCfg := m.customCfg.ForProject(projName)
	m.cmdItems = custom.FlatList(projCfg.Commands)
	// advance cursor to first non-header
	if m.cmdCursor == 0 || m.cmdCursor >= len(m.cmdItems) {
		for i, item := range m.cmdItems {
			if !item.IsHeader {
				m.cmdCursor = i
				break
			}
		}
	}
}

// ── Log tab keys ──────────────────────────────────────────────────────────────

func (m Model) handleLogKey(key string) (Model, tea.Cmd) {
	switch key {
	case "tab":
		return m.switchTab(tabProjects)
	case "up", "k":
		m.fullLogVP.LineUp(3)
	case "down", "j":
		m.fullLogVP.LineDown(3)
	case "pgup", "ctrl+u":
		m.fullLogVP.HalfViewUp()
	case "pgdown", "ctrl+d":
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
		c = exec.Command("powershell", "-NoProfile", "-Command", cmdStr)
	} else {
		c = exec.Command("bash", "-c", cmdStr)
	}
	c.Dir = cwd
	c.Stdin = os.Stdin
	return tea.ExecProcess(c, func(err error) tea.Msg {
		if err != nil {
			return flashMsg("⚠ 종료: " + err.Error())
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

	lines := git.StatusShort()
	m.files = nil
	for _, l := range lines {
		if len(l) < 3 {
			continue
		}
		m.files = append(m.files, fileEntry{xy: l[:2], path: strings.TrimSpace(l[3:])})
	}
	if m.fileCursor >= len(m.files) {
		if len(m.files) == 0 {
			m.fileCursor = 0
		} else {
			m.fileCursor = len(m.files) - 1
		}
	}
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
			if out2, _ := git.Run("show", ":"+f.path); out2 != "" {
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
	ch := m.contentHeight()
	lw := m.width * 30 / 100
	rw := m.width - lw - 1
	if rw < 10 {
		rw = 10
	}
	m.diffVP.Width = rw - 4
	m.diffVP.Height = ch - 10
	m.fullLogVP.Width = m.width - 6
	m.fullLogVP.Height = ch - 4
}

func (m Model) contentHeight() int {
	// status(1) + tabbar(1) + content(h) + buttonbar(1) + helpbar(1) = height
	h := m.height - 5
	if h < 5 {
		h = 5
	}
	return h
}

// ── Styles ────────────────────────────────────────────────────────────────────

var (
	styleBorderActive   = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("39"))
	styleBorderInactive = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("240"))
	styleHeader         = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("39"))
	styleStatusBar      = lipgloss.NewStyle().Background(lipgloss.Color("237")).Foreground(lipgloss.Color("255"))
	styleTabActive      = lipgloss.NewStyle().Background(lipgloss.Color("39")).Foreground(lipgloss.Color("0")).Bold(true).Padding(0, 1)
	styleTabInactive    = lipgloss.NewStyle().Background(lipgloss.Color("237")).Foreground(lipgloss.Color("252")).Padding(0, 1)
	styleFlash          = lipgloss.NewStyle().Foreground(lipgloss.Color("220"))
	styleStagedXY       = lipgloss.NewStyle().Foreground(lipgloss.Color("82"))
	styleUnstagedXY     = lipgloss.NewStyle().Foreground(lipgloss.Color("208"))
	styleUntrackedXY    = lipgloss.NewStyle().Foreground(lipgloss.Color("117"))
	styleSelected       = lipgloss.NewStyle().Background(lipgloss.Color("237")).Bold(true)
	styleDim            = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	styleGreen          = lipgloss.NewStyle().Foreground(lipgloss.Color("82"))
	styleRed            = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	styleYellow         = lipgloss.NewStyle().Foreground(lipgloss.Color("220"))
	styleCyan           = lipgloss.NewStyle().Foreground(lipgloss.Color("39"))
	styleGroupHdr       = lipgloss.NewStyle().Foreground(lipgloss.Color("75")).Bold(true)
	styleLogHash        = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	styleLogMsg         = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	stylePaletteBorder  = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("220"))
	stylePaletteInput   = lipgloss.NewStyle().Foreground(lipgloss.Color("255")).Bold(true)
	stylePaletteMatch   = lipgloss.NewStyle().Foreground(lipgloss.Color("220"))
)

// ── View ──────────────────────────────────────────────────────────────────────

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

	buttonBar := m.renderButtonBar()
	helpBar := m.renderHelpBar()
	base := lipgloss.JoinVertical(lipgloss.Left, statusBar, tabBar, content, buttonBar, helpBar)

	if m.paletteOpen {
		return m.renderPaletteOverlay(base)
	}
	return base
}

// ── Status bar ────────────────────────────────────────────────────────────────

func (m Model) renderStatusBar() string {
	left := "  " + styleHeader.Render("⎇  "+m.branch)
	if m.ahead != "" && m.ahead != "0" {
		left += styleGreen.Render(" ↑" + m.ahead)
	}
	if m.behind != "" && m.behind != "0" {
		left += styleRed.Render(" ↓" + m.behind)
	}
	if m.flash != "" {
		left += "  " + styleFlash.Render(m.flash)
	}

	right := ""
	if m.stashCount > 0 {
		right += styleYellow.Render(fmt.Sprintf("stash:%d  ", m.stashCount))
	}
	if m.flowLabel != "" {
		right += styleCyan.Render(m.flowLabel + "  ")
	}
	right += styleDim.Render("gez  ")

	gap := m.width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 0 {
		gap = 0
	}
	return styleStatusBar.Width(m.width).Render(left + strings.Repeat(" ", gap) + right)
}

// ── Tab bar ───────────────────────────────────────────────────────────────────

func (m Model) renderTabBar() string {
	defs := []struct {
		k string
		l string
		t tab
	}{
		{"1", "Projects", tabProjects},
		{"2", "Git", tabGit},
		{"3", "Commands", tabCommands},
		{"4", "Log", tabLog},
	}
	var parts []string
	for _, d := range defs {
		label := fmt.Sprintf("[%s] %s", d.k, d.l)
		if m.activeTab == d.t {
			parts = append(parts, styleTabActive.Render(label))
		} else {
			parts = append(parts, styleTabInactive.Render(label))
		}
	}
	hint := styleDim.Render("  ':'→ 모든 명령어")
	bar := " " + strings.Join(parts, " ") + hint
	pad := m.width - lipgloss.Width(bar)
	if pad > 0 {
		bar += styleDim.Render(strings.Repeat(" ", pad))
	}
	return bar
}

// ── Mouse click system ────────────────────────────────────────────────────────

// handleClick routes a left-click to the correct handler.
func (m Model) handleClick(x, y int) (Model, tea.Cmd) {
	// Palette is open — route to palette
	if m.paletteOpen {
		return m, nil
	}

	switch {
	case y == 1: // tab bar (row 1)
		return m.clickTabBar(x)
	case y == m.height-1: // button bar (last row)
		return m.clickBtnBar(x)
	case y >= 2 && y < m.height-1: // content area
		return m.clickContent(x, y)
	}
	return m, nil
}

// handleMouseScroll handles scroll wheel events.
func (m Model) handleMouseScroll(delta int) (Model, tea.Cmd) {
	switch {
	case m.activeTab == tabGit && m.gitPanel == panelDiff:
		if delta < 0 {
			m.diffVP.LineUp(-delta)
		} else {
			m.diffVP.LineDown(delta)
		}
	case m.activeTab == tabGit && m.gitPanel == panelFiles:
		if delta < 0 {
			return m.handleGitKey("up")
		}
		return m.handleGitKey("down")
	case m.activeTab == tabLog:
		if delta < 0 {
			m.fullLogVP.LineUp(-delta)
		} else {
			m.fullLogVP.LineDown(delta)
		}
	case m.activeTab == tabProjects:
		if delta < 0 {
			return m.handleProjectsKey("up")
		}
		return m.handleProjectsKey("down")
	case m.activeTab == tabCommands:
		if delta < 0 {
			m.moveCmdCursor(-1)
		} else {
			m.moveCmdCursor(1)
		}
	}
	return m, nil
}

// clickTabBar handles a click on the tab bar (y==1).
func (m Model) clickTabBar(x int) (Model, tea.Cmd) {
	type tabInfo struct {
		label string
		t     tab
	}
	tabs := []tabInfo{
		{"[1] Projects", tabProjects},
		{"[2] Git", tabGit},
		{"[3] Commands", tabCommands},
		{"[4] Log", tabLog},
	}
	px := 1 // skip leading space
	for _, t := range tabs {
		w := len(t.label) + 2 // Padding(0,1) adds 2 cols
		if x >= px && x < px+w {
			return m.switchTab(t.t)
		}
		px += w + 1 // +1 space between tabs
	}
	return m, nil
}

// clickBtnBar handles a click on the button bar (last row).
func (m Model) clickBtnBar(x int) (Model, tea.Cmd) {
	for _, z := range m.btnZones {
		if x >= z.x1 && x < z.x2 {
			return m.fireKey(z.key)
		}
	}
	return m, nil
}

// fireKey simulates pressing a key — dispatches through the normal key handlers.
func (m Model) fireKey(key string) (Model, tea.Cmd) {
	// global keys
	switch key {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "r":
		m.flash = "새로고침 중…"
		return m, tea.Batch(refresh(), loadProjects())
	case ":":
		m.paletteOpen = true
		m.paletteInput = ""
		m.paletteItems = allGezCommands
		m.paletteCursor = 0
		return m, nil
	case "1":
		return m.switchTab(tabProjects)
	case "2":
		return m.switchTab(tabGit)
	case "3":
		return m.switchTab(tabCommands)
	case "4":
		return m.switchTab(tabLog)
	}
	// tab-specific
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

// clickContent handles a click inside the content area (y>=2, y<height-1).
func (m Model) clickContent(x, y int) (Model, tea.Cmd) {
	switch m.activeTab {
	case tabProjects:
		return m.clickProjectsList(y)
	case tabGit:
		return m.clickGitContent(x, y)
	case tabCommands:
		return m.clickCommandsList(y)
	}
	return m, nil
}

// clickProjectsList selects a project on click, switches on second click.
func (m Model) clickProjectsList(y int) (Model, tea.Cmd) {
	// Panel starts at y=2 (content top), inside border y=3
	// Title "워크스페이스 프로젝트" at offset 0, blank at offset 1
	// Projects start at offset 2 → absolute y = 2+1+2 = 5
	const itemsStartY = 5
	idx := y - itemsStartY
	if idx < 0 || idx >= len(m.projEntries) {
		return m, nil
	}
	if m.lastClickY == y && m.lastClickItem == idx {
		// Double-click → activate
		m.lastClickY = -1
		m.lastClickItem = -1
		return m.handleProjectsKey("enter")
	}
	m.projCursor = idx
	m.lastClickY = y
	m.lastClickItem = idx
	return m, nil
}

// clickGitContent handles clicks in the git tab.
func (m Model) clickGitContent(x, y int) (Model, tea.Cmd) {
	lw := m.width*30/100 + 3 // panel width + border
	logH := 7
	mainH := m.contentHeight() - logH
	// File list panel: x in [0, lw), y in [2, 2+mainH)
	// Items start at y = 2+1 (top border) + 1 (title) + 1 (blank) = 5
	const itemsStartY = 5
	if x < lw && y >= itemsStartY && y < 2+mainH {
		idx := y - itemsStartY
		if idx < 0 || idx >= len(m.files) {
			return m, nil
		}
		m.gitPanel = panelFiles
		if m.lastClickY == y && m.lastClickItem == idx {
			// Double-click → stage/unstage
			m.lastClickY = -1
			m.lastClickItem = -1
			return m.handleGitKey(" ")
		}
		m.fileCursor = idx
		m.lastClickY = y
		m.lastClickItem = idx
		return m, m.loadDiffCmd()
	}
	return m, nil
}

// clickCommandsList selects a command on click, runs on second click.
func (m Model) clickCommandsList(y int) (Model, tea.Cmd) {
	// Panel at y=2+1 (border), title at 0, projName at 1, blank at 2
	// Items start at offset 3 → absolute y = 2+1+3 = 6
	const itemsStartY = 6
	idx := y - itemsStartY
	if idx < 0 || idx >= len(m.cmdItems) {
		return m, nil
	}
	if m.cmdItems[idx].IsHeader {
		return m, nil // can't click headers
	}
	if m.lastClickY == y && m.lastClickItem == idx {
		m.lastClickY = -1
		m.lastClickItem = -1
		m.cmdCursor = idx
		return m.handleCommandsKey("enter")
	}
	m.cmdCursor = idx
	m.lastClickY = y
	m.lastClickItem = idx
	return m, nil
}

// ── Button bar rendering & zone building ──────────────────────────────────────

var (
	styleBtnNormal  = lipgloss.NewStyle().Background(lipgloss.Color("237")).Foreground(lipgloss.Color("252")).Padding(0, 1)
	styleBtnBlue    = lipgloss.NewStyle().Background(lipgloss.Color("25")).Foreground(lipgloss.Color("255")).Bold(true).Padding(0, 1)
	styleBtnGreen   = lipgloss.NewStyle().Background(lipgloss.Color("28")).Foreground(lipgloss.Color("255")).Padding(0, 1)
	styleBtnOrange  = lipgloss.NewStyle().Background(lipgloss.Color("130")).Foreground(lipgloss.Color("255")).Padding(0, 1)
	styleBtnSep     = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
)

func (m *Model) rebuildBtnZones() {
	defs := m.currentTabBtns()
	var zones []btnZone
	x := 1
	for _, d := range defs {
		if d.sep {
			x += 3 // " │ "
			continue
		}
		w := len(d.label) + 2 // Padding(0,1)
		if x+w > m.width-2 {
			break
		}
		zones = append(zones, btnZone{x1: x, x2: x + w, key: d.key})
		x += w + 1
	}
	m.btnZones = zones
}

func (m Model) currentTabBtns() []tabBtnDef {
	sep := tabBtnDef{sep: true}
	n := styleBtnNormal
	b := styleBtnBlue
	g := styleBtnGreen
	o := styleBtnOrange

	switch m.activeTab {
	case tabGit:
		return []tabBtnDef{
			{label: "↕ stage", key: " ", style: n},
			{label: "⊕ all", key: "a", style: n},
			{label: "⊖ unstage", key: "u", style: n},
			sep,
			{label: "✦ commit", key: "c", style: b},
			{label: "↑ push", key: "p", style: b},
			{label: "↓ pull", key: "P", style: b},
			{label: "⟳ fetch", key: "f", style: n},
			{label: "⇅ sync", key: "S", style: n},
			sep,
			{label: "⎇ branch", key: "b", style: n},
			{label: "≡ stash", key: "s", style: o},
			{label: "⋈ merge", key: "m", style: n},
			{label: "⧸ rebase", key: "R", style: n},
			sep,
			{label: "⟳ refresh", key: "r", style: n},
			{label: "✕ quit", key: "q", style: n},
		}
	case tabProjects:
		return []tabBtnDef{
			{label: "⏎ switch", key: "enter", style: b},
			{label: "→ git", key: "g", style: n},
			{label: "→ cmds", key: "c", style: n},
			{label: "⚙ detect", key: "d", style: g},
			sep,
			{label: "⟳ refresh", key: "r", style: n},
			{label: "✕ quit", key: "q", style: n},
		}
	case tabCommands:
		return []tabBtnDef{
			{label: "▶ run", key: "enter", style: g},
			sep,
			{label: "＋ add", key: "A", style: b},
			{label: "🔍 detect", key: "D", style: b},
			{label: "✕ remove", key: "X", style: o},
			sep,
			{label: "⟳ refresh", key: "r", style: n},
			{label: "✕ quit", key: "q", style: n},
		}
	case tabLog:
		return []tabBtnDef{
			{label: "⏎ interactive", key: "enter", style: b},
			sep,
			{label: "⟳ refresh", key: "r", style: n},
			{label: "✕ quit", key: "q", style: n},
		}
	}
	return nil
}

// renderButtonBar renders the clickable button bar (bottom row).
func (m Model) renderButtonBar() string {
	defs := m.currentTabBtns()
	var parts []string
	x := 1
	for _, d := range defs {
		if d.sep {
			parts = append(parts, styleBtnSep.Render(" │"))
			x += 3
			continue
		}
		w := len(d.label) + 2
		if x+w > m.width-2 {
			break
		}
		rendered := d.style.Render(d.label)
		parts = append(parts, rendered)
		x += w + 1
	}
	bar := " " + strings.Join(parts, " ")
	// pad to full width
	pad := m.width - lipgloss.Width(bar)
	if pad > 0 {
		bar += styleDim.Render(strings.Repeat(" ", pad))
	}
	return bar
}

// ── Help bar (keyboard shortcut reminder, below button bar) ───────────────────

func (m Model) renderHelpBar() string {
	hint := ""
	switch m.activeTab {
	case tabProjects:
		hint = "↑↓/j/k:이동  enter:전환  g:Git  c:Cmds  d:감지  tab:탭전환  r:새로고침  q:종료"
	case tabGit:
		hint = "space:stage  a:all  h:hunk  c:commit  p:push  P:pull  b:branch  s:stash  R:rebase  l:로그  tab:패널전환"
	case tabCommands:
		hint = "↑↓:이동  enter:실행  A:추가  D:감지  X:삭제  tab:탭전환  r:새로고침  q:종료"
	case tabLog:
		hint = "↑↓/PgUp/PgDn:스크롤  enter:대화형  tab:탭전환  r:새로고침  q:종료"
	}
	return styleDim.Render("  " + hint)
}

// ── Tab 1: Projects ───────────────────────────────────────────────────────────

func (m Model) renderProjectsTab() string {
	h := m.contentHeight()
	lw := m.width * 38 / 100
	rw := m.width - lw - 1

	list := styleBorderActive.Width(lw).Height(h).Render(m.renderProjList(h - 2))
	detail := styleBorderInactive.Width(rw).Height(h).Render(m.renderProjDetail(h - 2))
	return lipgloss.JoinHorizontal(lipgloss.Top, list, detail)
}

func (m Model) renderProjList(max int) string {
	lines := []string{styleHeader.Render("워크스페이스 프로젝트"), ""}
	if !m.projReady {
		lines = append(lines, styleDim.Render("  로딩 중…"))
	} else if len(m.projEntries) == 0 {
		lines = append(lines, styleDim.Render("  (등록된 프로젝트 없음)"))
		lines = append(lines, "")
		lines = append(lines, styleDim.Render("  $ gez ws add"))
	} else {
		for i, e := range m.projEntries {
			sel := i == m.projCursor
			prefix := "  "
			nameStr := e.Name
			if sel {
				prefix = styleSelected.Render("▶ ")
				nameStr = styleSelected.Render(nameStr)
			} else {
				nameStr = styleCyan.Render(nameStr)
			}
			badge := ""
			if !e.Valid {
				badge = styleRed.Render(" ✗")
			} else if e.Changed > 0 {
				badge = styleYellow.Render(fmt.Sprintf(" ●%d", e.Changed))
			} else {
				badge = styleGreen.Render(" ✓")
			}
			lines = append(lines, prefix+nameStr+badge)
		}
	}
	for len(lines) < max {
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}

func (m Model) renderProjDetail(max int) string {
	if len(m.projEntries) == 0 || m.projCursor >= len(m.projEntries) {
		return styleDim.Render("\n  프로젝트를 선택하세요")
	}
	e := m.projEntries[m.projCursor]
	lines := []string{styleHeader.Render(e.Name), ""}
	if !e.Valid {
		lines = append(lines, styleRed.Render("  ⚠ 경로 없음 / git 저장소 아님"))
		lines = append(lines, styleDim.Render("  "+e.Path))
	} else {
		branch := styleCyan.Render("⎇  " + e.Branch)
		sync := ""
		if e.Ahead != "" && e.Ahead != "0" {
			sync += styleGreen.Render(" ↑" + e.Ahead)
		}
		if e.Behind != "" && e.Behind != "0" {
			sync += styleRed.Render(" ↓" + e.Behind)
		}
		lines = append(lines, "  "+branch+sync)
		lines = append(lines, "")
		lines = append(lines, styleDim.Render("  "+e.Path))
		lines = append(lines, "")
		if e.Changed == 0 {
			lines = append(lines, styleGreen.Render("  ✓ 깨끗"))
		} else {
			lines = append(lines, styleYellow.Render(fmt.Sprintf("  %d 파일 변경됨", e.Changed)))
		}
		lines = append(lines, "")
		lines = append(lines, styleDim.Render("  enter: 이 프로젝트로 전환"))
		lines = append(lines, styleDim.Render("  g: Git 탭 바로 이동"))
		lines = append(lines, styleDim.Render("  c: Commands 탭 바로 이동"))
	}
	for len(lines) < max {
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}

// ── Tab 2: Git ────────────────────────────────────────────────────────────────

func (m Model) renderGitTab() string {
	logH := 7
	h := m.contentHeight()
	mainH := h - logH
	if mainH < 4 {
		mainH = 4
	}
	lw := m.width * 30 / 100
	rw := m.width - lw - 1

	fb := styleBorderInactive
	if m.gitPanel == panelFiles {
		fb = styleBorderActive
	}
	db := styleBorderInactive
	if m.gitPanel == panelDiff {
		db = styleBorderActive
	}
	lb := styleBorderInactive
	if m.gitPanel == panelGitLog {
		lb = styleBorderActive
	}

	m.diffVP.Width = rw - 4
	m.diffVP.Height = mainH - 2

	filesPanel := fb.Width(lw).Height(mainH).Render(m.renderFiles(mainH - 2))
	diffPanel := db.Width(rw).Height(mainH).Render(m.diffVP.View())
	mainRow := lipgloss.JoinHorizontal(lipgloss.Top, filesPanel, diffPanel)
	logPanel := lb.Width(m.width - 2).Height(logH).Render(m.renderGitLog(logH - 2))

	return lipgloss.JoinVertical(lipgloss.Left, mainRow, logPanel)
}

func (m Model) renderFiles(max int) string {
	lines := []string{styleHeader.Render("변경 파일"), ""}
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
	for len(lines) < max {
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}

func (m Model) renderGitLog(max int) string {
	lines := []string{styleHeader.Render("최근 커밋"), ""}
	end := m.gitLogOff + max - 2
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
	lw := m.width * 40 / 100
	rw := m.width - lw - 1

	projName := currentProjectName()
	listContent := m.renderCmdList(projName, h-2)
	rightContent := m.renderCmdDetail(h - 2)

	left := styleBorderActive.Width(lw).Height(h).Render(listContent)
	right := styleBorderInactive.Width(rw).Height(h).Render(rightContent)
	return lipgloss.JoinHorizontal(lipgloss.Top, left, right)
}

func (m Model) renderCmdList(projName string, max int) string {
	lines := []string{
		styleHeader.Render("Commands"),
		styleDim.Render("  " + projName),
		"",
	}
	if len(m.cmdItems) == 0 {
		lines = append(lines, styleDim.Render("  (등록된 명령어 없음)"))
		lines = append(lines, "")
		lines = append(lines, styleDim.Render("  gez custom add"))
	}
	for i, item := range m.cmdItems {
		if item.IsHeader {
			lines = append(lines, styleGroupHdr.Render("  ── "+item.Header+" ──"))
			continue
		}
		sel := i == m.cmdCursor
		if sel {
			name := styleSelected.Render(fmt.Sprintf(" ▶ %-16s", item.Cmd.Name))
			desc := styleSelected.Render(item.Cmd.Description)
			lines = append(lines, name+" "+desc)
		} else {
			name := "   " + styleCyan.Render(fmt.Sprintf("%-16s", item.Cmd.Name))
			desc := styleDim.Render(item.Cmd.Description)
			lines = append(lines, name+" "+desc)
		}
	}
	for len(lines) < max {
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}

func (m Model) renderCmdDetail(max int) string {
	lines := []string{styleHeader.Render("실행 정보"), ""}

	if item := m.currentCmdItem(); item != nil {
		lines = append(lines, styleCyan.Render("  "+item.Cmd.Name))
		lines = append(lines, styleDim.Render("  "+item.Cmd.Description))
		lines = append(lines, "")

		cmdStr := item.Cmd.Cmd()
		if cmdStr != "" {
			lines = append(lines, styleDim.Render("  $ "+cmdStr))
			lines = append(lines, "")
			lines = append(lines, styleGreen.Render("  enter → 실행"))
		} else {
			lines = append(lines, styleRed.Render("  이 플랫폼에서 지원하지 않음"))
		}
	} else {
		lines = append(lines, styleDim.Render("  ↑↓ 로 명령어 선택"))
	}

	lines = append(lines, "")
	lines = append(lines, styleDim.Render(strings.Repeat("─", 30)))
	lines = append(lines, styleDim.Render("  명령어 관리"))
	lines = append(lines, styleDim.Render("  gez custom add  — 추가"))
	lines = append(lines, styleDim.Render("  gez custom rm   — 삭제"))
	lines = append(lines, styleDim.Render("  gez custom ls   — 전체 목록"))

	for len(lines) < max {
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}

// ── Tab 4: Log ────────────────────────────────────────────────────────────────

func (m Model) renderLogTab() string {
	h := m.contentHeight()
	m.fullLogVP.Width = m.width - 6
	m.fullLogVP.Height = h - 4

	title := styleHeader.Render("커밋 로그 (최근 100개)")
	hint := styleDim.Render("  enter / i : 대화형 선택 → show · cherry-pick · reset")
	inner := lipgloss.JoinVertical(lipgloss.Left, title, "", m.fullLogVP.View(), "", hint)
	return styleBorderActive.Width(m.width-2).Height(h).Render(inner)
}

// ── Command Palette overlay ───────────────────────────────────────────────────

func (m Model) renderPaletteOverlay(base string) string {
	pw := m.width * 70 / 100
	if pw < 50 {
		pw = 50
	}
	if pw > m.width-4 {
		pw = m.width - 4
	}
	ph := 20
	if ph > m.height-6 {
		ph = m.height - 6
	}
	if ph < 5 {
		ph = 5
	}

	// Input line
	inputLine := stylePaletteInput.Render(": " + m.paletteInput + "█")

	// Item list (scrolling window)
	maxItems := ph - 4
	if maxItems < 1 {
		maxItems = 1
	}
	start := 0
	if m.paletteCursor > maxItems-1 {
		start = m.paletteCursor - maxItems + 1
	}
	end := start + maxItems
	if end > len(m.paletteItems) {
		end = len(m.paletteItems)
	}

	var itemLines []string
	for i := start; i < end; i++ {
		item := m.paletteItems[i]
		if i == m.paletteCursor {
			line := styleSelected.Render(fmt.Sprintf(" ▶ %-20s  %s", item.name, item.desc))
			itemLines = append(itemLines, line)
		} else {
			line := "   " + stylePaletteMatch.Render(fmt.Sprintf("%-20s", item.name)) + "  " + styleDim.Render(item.desc)
			itemLines = append(itemLines, line)
		}
	}
	if len(m.paletteItems) == 0 {
		itemLines = append(itemLines, styleDim.Render("  (일치하는 명령어 없음)"))
	}

	sep := styleDim.Render(strings.Repeat("─", pw-4))
	countStr := ""
	if len(m.paletteItems) > 0 {
		countStr = fmt.Sprintf("  [%d/%d]", m.paletteCursor+1, len(m.paletteItems))
	}
	hint := styleDim.Render("  enter:실행  ↑↓:이동  esc:닫기  backspace:수정" + countStr)

	inner := lipgloss.JoinVertical(lipgloss.Left,
		inputLine, sep,
		strings.Join(itemLines, "\n"),
		sep, hint,
	)

	paletteBox := stylePaletteBorder.Width(pw).Render(inner)

	// Overlay: place palette centered on the full terminal.
	// The base content shows through on terminals that support it;
	// whitespace is filled with the dim background colour.
	_ = base // base content is visible through transparent whitespace on most terminals
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, paletteBox)
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
	addS := lipgloss.NewStyle().Foreground(lipgloss.Color("82"))
	delS := lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	hunkS := lipgloss.NewStyle().Foreground(lipgloss.Color("39"))
	hdrS := lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Bold(true)
	var sb strings.Builder
	for _, line := range strings.Split(s, "\n") {
		switch {
		case strings.HasPrefix(line, "+++") || strings.HasPrefix(line, "---"):
			sb.WriteString(hdrS.Render(line))
		case strings.HasPrefix(line, "@@"):
			sb.WriteString(hunkS.Render(line))
		case strings.HasPrefix(line, "+"):
			sb.WriteString(addS.Render(line))
		case strings.HasPrefix(line, "-"):
			sb.WriteString(delS.Render(line))
		default:
			sb.WriteString(line)
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

func colorizeLog(s string) string {
	hashS := lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	var sb strings.Builder
	for _, line := range strings.Split(s, "\n") {
		if idx := strings.Index(line, " "); idx > 0 {
			sb.WriteString(hashS.Render(line[:idx]) + line[idx:])
		} else {
			sb.WriteString(line)
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// ── All gez commands (command palette) ───────────────────────────────────────

var allGezCommands = []paletteItem{
	// 기본 워크플로우
	{name: "commit", args: []string{"commit"}, desc: "커밋 마법사 (Conventional Commits)"},
	{name: "push", args: []string{"push"}, desc: "원격 푸시"},
	{name: "push -f", args: []string{"push", "-f"}, desc: "force-with-lease 강제 푸시"},
	{name: "pull", args: []string{"pull"}, desc: "원격 풀"},
	{name: "sync", args: []string{"sync"}, desc: "fetch + pull"},
	{name: "fetch", args: []string{"fetch"}, desc: "fetch --all --prune"},
	{name: "status", args: []string{"status"}, desc: "현재 상태 상세"},
	{name: "diff", args: []string{"diff"}, desc: "변경사항 diff"},
	{name: "log", args: []string{"log"}, desc: "커밋 로그"},
	{name: "log -i", args: []string{"log", "-i"}, desc: "대화형 로그 (show·cherry-pick·reset)"},
	// 브랜치 & 히스토리
	{name: "branch", args: []string{"branch"}, desc: "브랜치 관리 메뉴"},
	{name: "merge", args: []string{"merge"}, desc: "브랜치 병합"},
	{name: "rebase", args: []string{"rebase"}, desc: "리베이스 (/ -i interactive)"},
	{name: "cherry-pick", args: []string{"cherry-pick"}, desc: "다른 브랜치 커밋 가져오기"},
	{name: "revert", args: []string{"revert"}, desc: "커밋 되돌리기 (히스토리 유지)"},
	{name: "reset", args: []string{"reset"}, desc: "soft·mixed·hard reset"},
	// 커밋 관리
	{name: "squash", args: []string{"squash"}, desc: "최근 N개 커밋 합치기"},
	{name: "amend", args: []string{"amend"}, desc: "마지막 커밋 수정"},
	{name: "fixup", args: []string{"fixup"}, desc: "fixup 커밋 + autosquash"},
	{name: "undo", args: []string{"undo"}, desc: "마지막 작업 취소 (reflog 기반)"},
	{name: "restore", args: []string{"restore"}, desc: "파일 복원"},
	{name: "changelog", args: []string{"changelog"}, desc: "CHANGELOG.md 생성"},
	// 복구 & 정리
	{name: "stash", args: []string{"stash"}, desc: "스태시 push·pop·apply·drop"},
	{name: "reflog", args: []string{"reflog"}, desc: "reflog 조회 + 복구"},
	{name: "blame", args: []string{"blame"}, desc: "줄별 작성자·커밋"},
	{name: "clean", args: []string{"clean"}, desc: "untracked 파일 정리"},
	// 검색 & 분석
	{name: "search", args: []string{"search"}, desc: "메시지·pickaxe·regex·grep·파일명 검색"},
	{name: "show", args: []string{"show"}, desc: "커밋 상세 보기"},
	{name: "stats", args: []string{"stats"}, desc: "저장소 통계 (기여자·파일·월별)"},
	{name: "file", args: []string{"file"}, desc: "파일별 히스토리·blame·복원"},
	{name: "bisect", args: []string{"bisect"}, desc: "이진 탐색으로 버그 커밋 찾기"},
	// 저장소 & 원격
	{name: "tag", args: []string{"tag"}, desc: "태그 생성·삭제·push"},
	{name: "remote", args: []string{"remote"}, desc: "원격 저장소 관리"},
	{name: "init", args: []string{"init"}, desc: "새 git 저장소 초기화"},
	{name: "clone", args: []string{"clone"}, desc: "저장소 클론"},
	{name: "worktree", args: []string{"worktree"}, desc: "워크트리 관리"},
	{name: "submodule", args: []string{"submodule"}, desc: "서브모듈 관리"},
	{name: "pr", args: []string{"pr"}, desc: "PR/MR URL 브라우저 열기"},
	{name: "hook", args: []string{"hook"}, desc: "Git hooks 관리"},
	{name: "config", args: []string{"config"}, desc: "Git + gez 설정"},
	{name: "archive", args: []string{"archive"}, desc: "zip·tar.gz 내보내기"},
	{name: "patch", args: []string{"patch"}, desc: "패치 생성·적용"},
	{name: "sparse", args: []string{"sparse"}, desc: "Sparse checkout (모노레포)"},
	// 환경 설정
	{name: "ignore", args: []string{"ignore"}, desc: ".gitignore 관리 (12종 템플릿)"},
	{name: "alias", args: []string{"alias"}, desc: "git alias 관리"},
	{name: "doctor", args: []string{"doctor"}, desc: "Git 환경 진단"},
	{name: "completion-install", args: []string{"completion-install"}, desc: "쉘 자동완성 설치"},
	// Flow
	{name: "flow", args: []string{"flow"}, desc: "Flow 전략 현황 + 힌트"},
	{name: "flow init", args: []string{"flow", "init"}, desc: "Flow 전략 초기화"},
	{name: "flow feature start", args: []string{"flow", "feature", "start"}, desc: "feature 브랜치 시작"},
	{name: "flow feature finish", args: []string{"flow", "feature", "finish"}, desc: "feature 완료"},
	{name: "flow release start", args: []string{"flow", "release", "start"}, desc: "release 브랜치 시작"},
	{name: "flow release finish", args: []string{"flow", "release", "finish"}, desc: "release 완료"},
	{name: "flow hotfix start", args: []string{"flow", "hotfix", "start"}, desc: "hotfix 시작"},
	{name: "flow hotfix finish", args: []string{"flow", "hotfix", "finish"}, desc: "hotfix 완료"},
	// 워크스페이스
	{name: "ws", args: []string{"ws"}, desc: "워크스페이스 전체 상태"},
	{name: "ws add", args: []string{"ws", "add"}, desc: "프로젝트 등록"},
	{name: "ws pull", args: []string{"ws", "pull"}, desc: "전체 프로젝트 pull"},
	{name: "ws sync", args: []string{"ws", "sync"}, desc: "전체 프로젝트 fetch + pull"},
	// 커스텀
	{name: "custom", args: []string{"custom"}, desc: "커스텀 명령어 관리"},
	{name: "custom detect", args: []string{"custom", "detect"}, desc: "프로젝트 파일 자동 분석 → 명령어 등록"},
	{name: "custom add", args: []string{"custom", "add"}, desc: "커스텀 명령어 추가"},
	{name: "custom ls", args: []string{"custom", "ls"}, desc: "커스텀 명령어 목록"},
	{name: "custom rm", args: []string{"custom", "rm"}, desc: "커스텀 명령어 삭제"},
}
