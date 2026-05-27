// Package tui implements a full-screen terminal UI for gez using bubbletea.
package tui

import (
	"fmt"
	"gez/internal/git"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ── Panels ────────────────────────────────────────────────────────────────────

type panel int

const (
	panelFiles panel = iota
	panelDiff
	panelLog
)

// ── File entry ────────────────────────────────────────────────────────────────

type fileEntry struct {
	xy   string // two-char git status XY
	path string
}

func (f fileEntry) staged() bool   { return f.xy[0] != ' ' && f.xy[0] != '?' }
func (f fileEntry) unstaged() bool { return f.xy[1] != ' ' }

// ── Messages ─────────────────────────────────────────────────────────────────

type refreshMsg struct{}
type diffLoadedMsg string

// ── Model ─────────────────────────────────────────────────────────────────────

type Model struct {
	width, height int
	activePanel   panel

	// Files panel
	files    []fileEntry
	fileCursor int

	// Diff panel
	diffVP   viewport.Model
	diffReady bool

	// Log panel
	logLines  []string
	logOffset int

	// Status bar
	branch string
	ahead  string
	behind string

	// Transient message at bottom
	flash string
}

// New creates an initialised Model.
func New() Model {
	vp := viewport.New(0, 0)
	vp.Style = lipgloss.NewStyle()
	return Model{
		diffVP:      vp,
		activePanel: panelFiles,
	}
}

// ── Init ──────────────────────────────────────────────────────────────────────

func (m Model) Init() tea.Cmd {
	return refresh()
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
		m.loadFiles()
		m.loadLog()
		m.branch = git.CurrentBranch()
		m.ahead, m.behind = git.AheadBehind()
		m.flash = ""
		return m, m.loadDiffCmd()

	case diffLoadedMsg:
		m.diffVP.SetContent(string(msg))
		m.diffReady = true
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	// Forward scroll events to active viewport
	if m.activePanel == panelDiff {
		var cmd tea.Cmd
		m.diffVP, cmd = m.diffVP.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m *Model) handleKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	key := msg.String()

	switch key {
	case "q", "ctrl+c":
		return *m, tea.Quit

	case "tab":
		m.activePanel = (m.activePanel + 1) % 3
		return *m, nil

	case "r":
		m.flash = "새로고침 중…"
		return *m, refresh()

	// File navigation
	case "up", "k":
		if m.activePanel == panelFiles && m.fileCursor > 0 {
			m.fileCursor--
			return *m, m.loadDiffCmd()
		}
		if m.activePanel == panelLog && m.logOffset > 0 {
			m.logOffset--
		}

	case "down", "j":
		if m.activePanel == panelFiles && m.fileCursor < len(m.files)-1 {
			m.fileCursor++
			return *m, m.loadDiffCmd()
		}
		if m.activePanel == panelLog {
			maxOff := len(m.logLines) - logVisible(m.height)
			if maxOff < 0 {
				maxOff = 0
			}
			if m.logOffset < maxOff {
				m.logOffset++
			}
		}

	// Stage / unstage
	case " ":
		if m.activePanel == panelFiles && len(m.files) > 0 {
			f := m.files[m.fileCursor]
			var err error
			if f.staged() {
				_, err = git.Run("restore", "--staged", f.path)
				if err != nil {
					m.flash = "unstage 실패: " + err.Error()
				} else {
					m.flash = fmt.Sprintf("✔ unstaged: %s", f.path)
				}
			} else {
				_, err = git.Run("add", f.path)
				if err != nil {
					m.flash = "stage 실패: " + err.Error()
				} else {
					m.flash = fmt.Sprintf("✔ staged: %s", f.path)
				}
			}
			return *m, refresh()
		}

	case "a":
		// stage all
		if _, err := git.Run("add", "-A"); err != nil {
			m.flash = "stage all 실패: " + err.Error()
		} else {
			m.flash = "✔ 전체 staged"
		}
		return *m, refresh()

	case "u":
		// unstage all
		if _, err := git.Run("restore", "--staged", "."); err != nil {
			m.flash = "unstage all 실패"
		} else {
			m.flash = "✔ 전체 unstaged"
		}
		return *m, refresh()

	// Diff scroll when in diff panel
	case "pgup", "ctrl+u":
		m.diffVP.HalfViewUp()
	case "pgdown", "ctrl+d":
		m.diffVP.HalfViewDown()
	}

	return *m, nil
}

// ── Data loading ──────────────────────────────────────────────────────────────

func (m *Model) loadFiles() {
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
		m.fileCursor = max(0, len(m.files)-1)
	}
}

func (m *Model) loadLog() {
	m.logLines = git.RecentCommits(80)
}

func (m Model) loadDiffCmd() tea.Cmd {
	if len(m.files) == 0 {
		return func() tea.Msg { return diffLoadedMsg("변경된 파일 없음") }
	}
	f := m.files[m.fileCursor]
	return func() tea.Msg {
		var out string
		var err error
		if f.staged() {
			out, err = git.Run("diff", "--cached", "--", f.path)
		} else {
			out, err = git.Run("diff", "--", f.path)
		}
		if err != nil || out == "" {
			// Untracked file: show cat-like output
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

// ── Layout helpers ────────────────────────────────────────────────────────────

func (m *Model) resizePanels() {
	// Layout: left 30% files | right 70% diff  (full width)
	//         bottom log strip (fixed 8 lines)
	logH := 8
	mainH := m.height - logH - 3 // 3 = borders/statusbar
	if mainH < 5 {
		mainH = 5
	}
	leftW := m.width * 30 / 100
	rightW := m.width - leftW - 3
	if rightW < 10 {
		rightW = 10
	}
	m.diffVP.Width = rightW
	m.diffVP.Height = mainH
}

func logVisible(h int) int {
	v := 8 - 2
	if v < 1 {
		v = 1
	}
	return v
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// ── View ──────────────────────────────────────────────────────────────────────

var (
	styleBorderActive   = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("39"))
	styleBorderInactive = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("240"))
	styleHeader         = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("39"))
	styleStatusBar      = lipgloss.NewStyle().Background(lipgloss.Color("237")).Foreground(lipgloss.Color("255")).PaddingLeft(1).PaddingRight(1)
	styleFlash          = lipgloss.NewStyle().Foreground(lipgloss.Color("220"))
	styleStagedXY       = lipgloss.NewStyle().Foreground(lipgloss.Color("82"))
	styleUnstagedXY     = lipgloss.NewStyle().Foreground(lipgloss.Color("208"))
	styleUntrackedXY    = lipgloss.NewStyle().Foreground(lipgloss.Color("117"))
	styleSelected       = lipgloss.NewStyle().Background(lipgloss.Color("237")).Bold(true)
	styleDimText        = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	styleLogHash        = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	styleLogMsg         = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
)

func (m Model) View() string {
	if m.width == 0 {
		return "로딩 중…"
	}

	logH := 8
	mainH := m.height - logH - 3
	if mainH < 5 {
		mainH = 5
	}
	leftW := m.width * 30 / 100
	rightW := m.width - leftW - 1

	// ── Files panel ───────────────────────────────────────────────────────────
	filesContent := m.renderFiles(mainH - 2)
	filesBorder := styleBorderInactive
	if m.activePanel == panelFiles {
		filesBorder = styleBorderActive
	}
	filesPanel := filesBorder.Width(leftW).Height(mainH).Render(filesContent)

	// ── Diff panel ────────────────────────────────────────────────────────────
	diffBorder := styleBorderInactive
	if m.activePanel == panelDiff {
		diffBorder = styleBorderActive
	}
	m.diffVP.Width = rightW - 4
	m.diffVP.Height = mainH - 2
	diffPanel := diffBorder.Width(rightW).Height(mainH).Render(m.diffVP.View())

	// ── Main row ──────────────────────────────────────────────────────────────
	mainRow := lipgloss.JoinHorizontal(lipgloss.Top, filesPanel, diffPanel)

	// ── Log panel ─────────────────────────────────────────────────────────────
	logBorder := styleBorderInactive
	if m.activePanel == panelLog {
		logBorder = styleBorderActive
	}
	logContent := m.renderLog(logH - 2)
	logPanel := logBorder.Width(m.width - 2).Height(logH).Render(logContent)

	// ── Status bar ────────────────────────────────────────────────────────────
	statusBar := m.renderStatusBar()

	// ── Help bar ─────────────────────────────────────────────────────────────
	helpBar := styleDimText.Render("  space:stage/unstage  a:all  u:unstage-all  r:새로고침  tab:패널이동  q:종료")

	return lipgloss.JoinVertical(lipgloss.Left,
		statusBar,
		mainRow,
		logPanel,
		helpBar,
	)
}

func (m Model) renderStatusBar() string {
	branch := styleHeader.Render("⎇  " + m.branch)
	sync := ""
	if m.ahead != "" && m.ahead != "0" {
		sync += lipgloss.NewStyle().Foreground(lipgloss.Color("82")).Render(fmt.Sprintf(" ↑%s", m.ahead))
	}
	if m.behind != "" && m.behind != "0" {
		sync += lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Render(fmt.Sprintf(" ↓%s", m.behind))
	}
	flash := ""
	if m.flash != "" {
		flash = "  " + styleFlash.Render(m.flash)
	}
	left := "  " + branch + sync + flash
	right := styleDimText.Render("gez TUI  ")
	gap := m.width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 0 {
		gap = 0
	}
	return styleStatusBar.Width(m.width).Render(left + strings.Repeat(" ", gap) + right)
}

func (m Model) renderFiles(maxLines int) string {
	header := styleHeader.Render("변경 파일")
	lines := []string{header, ""}

	if len(m.files) == 0 {
		lines = append(lines, styleDimText.Render("  (변경 없음)"))
	}

	for i, f := range m.files {
		xy := xyStyle(f.xy).Render(f.xy)
		path := f.path
		if i == m.fileCursor && m.activePanel == panelFiles {
			path = styleSelected.Render("▶ " + path)
		} else {
			path = "  " + path
		}
		lines = append(lines, fmt.Sprintf(" %s %s", xy, path))
	}

	// pad to fill
	for len(lines) < maxLines {
		lines = append(lines, "")
	}

	return strings.Join(lines, "\n")
}

func xyStyle(xy string) lipgloss.Style {
	if len(xy) < 2 {
		return styleDimText
	}
	if xy[0] != ' ' && xy[0] != '?' {
		return styleStagedXY
	}
	if xy == "??" {
		return styleUntrackedXY
	}
	return styleUnstagedXY
}

func (m Model) renderLog(maxLines int) string {
	header := styleHeader.Render("최근 커밋")
	lines := []string{header, ""}

	end := m.logOffset + maxLines - 2
	if end > len(m.logLines) {
		end = len(m.logLines)
	}

	for _, l := range m.logLines[m.logOffset:end] {
		parts := strings.SplitN(l, " ", 2)
		if len(parts) == 2 {
			lines = append(lines, styleLogHash.Render(parts[0])+"  "+styleLogMsg.Render(parts[1]))
		} else {
			lines = append(lines, l)
		}
	}
	return strings.Join(lines, "\n")
}

// colorizeDiff adds lipgloss colour to unified diff output.
func colorizeDiff(s string) string {
	addStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("82"))
	delStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	hunkStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("39"))
	headerStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Bold(true)

	var sb strings.Builder
	for _, line := range strings.Split(s, "\n") {
		switch {
		case strings.HasPrefix(line, "+++") || strings.HasPrefix(line, "---"):
			sb.WriteString(headerStyle.Render(line))
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
