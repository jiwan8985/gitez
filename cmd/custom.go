package cmd

import (
	"fmt"
	"gez/internal/custom"
	"gez/internal/ui"
	"gez/internal/workspace"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"
)

var customCmd = &cobra.Command{
	Use:   "custom",
	Short: "프로젝트별 커스텀 명령어 관리",
	Long:  `프로젝트별 커스텀 명령어를 추가·삭제·실행합니다. TUI [3] Commands 탭에 표시됩니다.`,
	Run: func(cmd *cobra.Command, args []string) {
		runCustomMenu()
	},
}

var customListCmd = &cobra.Command{
	Use:     "list [프로젝트]",
	Aliases: []string{"ls"},
	Short:   "커스텀 명령어 목록",
	Run: func(cmd *cobra.Command, args []string) {
		cfg := custom.Load()
		projName := customCurrentProject()
		if len(args) > 0 {
			projName = args[0]
		}
		runCustomList(cfg, projName)
	},
}

var customAddCmd = &cobra.Command{
	Use:   "add",
	Short: "커스텀 명령어 추가",
	Run:   func(cmd *cobra.Command, args []string) { runCustomAdd() },
}

var customRemoveCmd = &cobra.Command{
	Use:     "remove",
	Aliases: []string{"rm", "del"},
	Short:   "커스텀 명령어 삭제",
	Run:     func(cmd *cobra.Command, args []string) { runCustomRemove() },
}

var customRunCmd = &cobra.Command{
	Use:   "run <이름>",
	Short: "커스텀 명령어 실행",
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		cfg := custom.Load()
		projName := customCurrentProject()
		projCfg := cfg.ForProject(projName)
		name := args[0]
		for _, c := range projCfg.Commands {
			if c.Name == name {
				runCustomExec(c)
				return
			}
		}
		ui.Fail(fmt.Sprintf("'%s' 명령어를 찾을 수 없습니다 (프로젝트: %s)", name, projName))
		fmt.Printf("  %s\n", ui.Dim("gez custom ls  →  전체 목록 확인"))
		os.Exit(1)
	},
}

func init() {
	customCmd.AddCommand(customListCmd)
	customCmd.AddCommand(customAddCmd)
	customCmd.AddCommand(customRemoveCmd)
	customCmd.AddCommand(customRunCmd)
	rootCmd.AddCommand(customCmd)
}

// ── Interactive menu ──────────────────────────────────────────────────────────

func runCustomMenu() {
	fmt.Println()
	fmt.Printf("  %s\n\n", ui.Bold("커스텀 명령어"))

	cfg := custom.Load()
	projName := customCurrentProject()
	projCfg := cfg.ForProject(projName)

	fmt.Printf("  %s  %s\n\n", ui.Dim("프로젝트:"), ui.BoldCyan(projName))

	actions := []string{
		"명령어 실행",
		"명령어 추가",
		"명령어 삭제",
		"다른 프로젝트 보기",
		"설정 파일 경로 보기",
		"취소",
	}
	var action string
	if err := survey.AskOne(&survey.Select{
		Message: "작업 선택:",
		Options: actions,
	}, &action); err != nil || action == "취소" {
		return
	}

	fmt.Println()
	switch action {
	case "명령어 실행":
		runCustomRunMenu(projCfg)
	case "명령어 추가":
		runCustomAdd()
	case "명령어 삭제":
		runCustomRemove()
	case "다른 프로젝트 보기":
		var names []string
		for _, p := range cfg.Projects {
			names = append(names, p.Name)
		}
		var selected string
		if err := survey.AskOne(&survey.Select{
			Message: "프로젝트:",
			Options: names,
		}, &selected); err == nil {
			runCustomList(cfg, selected)
		}
	case "설정 파일 경로 보기":
		fmt.Printf("  %s\n\n", ui.Cyan(custom.ConfigPath()))
	}
}

func runCustomRunMenu(projCfg custom.ProjectConfig) {
	if len(projCfg.Commands) == 0 {
		ui.Fail("등록된 명령어가 없습니다")
		fmt.Printf("  %s\n", ui.Dim("gez custom add  →  명령어 추가"))
		return
	}

	options := make([]string, len(projCfg.Commands))
	for i, c := range projCfg.Commands {
		options[i] = fmt.Sprintf("%-20s %s", c.Name, c.Description)
	}
	var selected string
	if err := survey.AskOne(&survey.Select{
		Message: "실행할 명령어:",
		Options: options,
	}, &selected); err != nil {
		return
	}

	for _, c := range projCfg.Commands {
		if strings.HasPrefix(selected, c.Name) {
			fmt.Println()
			runCustomExec(c)
			return
		}
	}
}

// ── List ──────────────────────────────────────────────────────────────────────

func runCustomList(cfg custom.Config, projName string) {
	projCfg := cfg.ForProject(projName)
	fmt.Println()
	fmt.Printf("  %s  %s\n\n", ui.Bold("커스텀 명령어"), ui.BoldCyan(projName))

	if len(projCfg.Commands) == 0 {
		fmt.Printf("  %s\n\n", ui.Dim("(등록된 명령어 없음)"))
		fmt.Printf("  %s\n\n", ui.Dim("gez custom add  →  명령어 추가"))
		return
	}

	for _, g := range custom.GroupCommands(projCfg.Commands) {
		fmt.Printf("  %s\n", ui.Bold(g.Group))
		for _, c := range g.Commands {
			cmdStr := c.Cmd()
			if cmdStr == "" {
				cmdStr = "(이 플랫폼 미지원)"
			}
			fmt.Printf("    %s  %s\n",
				ui.Cyan(fmt.Sprintf("%-18s", c.Name)),
				ui.Dim(c.Description))
			fmt.Printf("    %s  %s\n",
				strings.Repeat(" ", 18+4),
				ui.Dim("$ "+cmdStr))
		}
		fmt.Println()
	}
	fmt.Printf("  %s  %s\n\n",
		ui.Dim("설정 파일:"),
		ui.Dim(custom.ConfigPath()))
}

// ── Add ───────────────────────────────────────────────────────────────────────

func runCustomAdd() {
	fmt.Println()
	fmt.Printf("  %s\n\n", ui.Bold("커스텀 명령어 추가"))

	cfg := custom.Load()

	// Project selection
	projNames := make([]string, 0, len(cfg.Projects)+1)
	for _, p := range cfg.Projects {
		projNames = append(projNames, p.Name)
	}
	projNames = append(projNames, "(새 프로젝트 입력)")

	defProj := customCurrentProject()
	var projName string
	if err := survey.AskOne(&survey.Select{
		Message: "프로젝트:",
		Options: projNames,
		Default: defProj,
	}, &projName); err != nil {
		return
	}
	if projName == "(새 프로젝트 입력)" {
		if err := survey.AskOne(&survey.Input{
			Message: "프로젝트 이름:",
		}, &projName, survey.WithValidator(survey.Required)); err != nil {
			return
		}
	}

	var name, desc, group, cmdWin, cmdUnix string
	qs := []*survey.Question{
		{Name: "name", Prompt: &survey.Input{Message: "명령어 이름 (예: dev, build, deploy):"}, Validate: survey.Required},
		{Name: "desc", Prompt: &survey.Input{Message: "설명:"}},
		{Name: "group", Prompt: &survey.Input{Message: "그룹 (서비스/빌드/품질/기타 등):", Default: "기타"}},
	}
	answers := struct {
		Name  string `survey:"name"`
		Desc  string `survey:"desc"`
		Group string `survey:"group"`
	}{}
	if err := survey.Ask(qs, &answers); err != nil {
		return
	}
	name = strings.TrimSpace(answers.Name)
	desc = strings.TrimSpace(answers.Desc)
	group = strings.TrimSpace(answers.Group)

	if err := survey.AskOne(&survey.Input{
		Message: "Windows 명령어 (PowerShell, 비우면 Unix와 동일):",
		Help:    "예: .\\make.ps1 dev  또는  docker compose up -d",
	}, &cmdWin); err != nil {
		return
	}
	if err := survey.AskOne(&survey.Input{
		Message: "Unix 명령어 (bash):",
		Help:    "예: make dev  또는  docker compose up -d",
	}, &cmdUnix); err != nil {
		return
	}

	cmdWin = strings.TrimSpace(cmdWin)
	cmdUnix = strings.TrimSpace(cmdUnix)
	if cmdWin == "" {
		cmdWin = cmdUnix
	}
	if cmdUnix == "" {
		cmdUnix = cmdWin
	}

	newCmd := custom.Command{
		Name:        name,
		Description: desc,
		Group:       group,
		CmdWin:      cmdWin,
		CmdUnix:     cmdUnix,
	}

	cfg.AddCommand(projName, newCmd)
	if err := custom.Save(cfg); err != nil {
		ui.Fail("저장 실패: " + err.Error())
		return
	}
	fmt.Println()
	ui.Success(fmt.Sprintf("'%s' 명령어가 [%s]에 추가됐습니다", newCmd.Name, projName))
	fmt.Printf("  %s  %s\n\n", ui.Dim("TUI 확인:"), ui.Cyan("gez ui  →  [3] 키로 Commands 탭"))
}

// ── Remove ────────────────────────────────────────────────────────────────────

func runCustomRemove() {
	fmt.Println()
	cfg := custom.Load()

	projNames := make([]string, len(cfg.Projects))
	for i, p := range cfg.Projects {
		projNames[i] = p.Name
	}
	defProj := customCurrentProject()

	var projName string
	if err := survey.AskOne(&survey.Select{
		Message: "프로젝트:",
		Options: projNames,
		Default: defProj,
	}, &projName); err != nil {
		return
	}

	projCfg := cfg.ForProject(projName)
	if len(projCfg.Commands) == 0 {
		ui.Fail("등록된 명령어가 없습니다")
		return
	}

	options := make([]string, len(projCfg.Commands))
	for i, c := range projCfg.Commands {
		options[i] = fmt.Sprintf("%-20s %s", c.Name, c.Description)
	}
	var selected string
	if err := survey.AskOne(&survey.Select{
		Message: "삭제할 명령어:",
		Options: options,
	}, &selected); err != nil {
		return
	}

	cmdName := strings.Fields(selected)[0]

	var ok bool
	if err := survey.AskOne(&survey.Confirm{
		Message: fmt.Sprintf("'%s' 명령어를 삭제할까요?", cmdName),
		Default: false,
	}, &ok); err != nil || !ok {
		return
	}

	if cfg.RemoveCommand(projName, cmdName) {
		if err := custom.Save(cfg); err != nil {
			ui.Fail("저장 실패: " + err.Error())
			return
		}
		fmt.Println()
		ui.Success(fmt.Sprintf("'%s' 명령어가 삭제됐습니다", cmdName))
	} else {
		ui.Fail("명령어를 찾을 수 없습니다")
	}
	fmt.Println()
}

// ── Exec ──────────────────────────────────────────────────────────────────────

func runCustomExec(c custom.Command) {
	cmdStr := c.Cmd()
	if cmdStr == "" {
		ui.Fail("이 플랫폼에서 지원하지 않는 명령어입니다")
		return
	}
	cwd, _ := os.Getwd()
	fmt.Printf("  %s  %s\n\n", ui.Dim("실행:"), ui.Cyan(cmdStr))

	var shellCmd *exec.Cmd
	if runtime.GOOS == "windows" {
		shellCmd = exec.Command("powershell", "-NoProfile", "-Command", cmdStr)
	} else {
		shellCmd = exec.Command("bash", "-c", cmdStr)
	}
	shellCmd.Dir = cwd
	shellCmd.Stdout = os.Stdout
	shellCmd.Stderr = os.Stderr
	shellCmd.Stdin = os.Stdin
	if err := shellCmd.Run(); err != nil {
		ui.Fail("명령 종료: " + err.Error())
	}
}

// ── Helpers ───────────────────────────────────────────────────────────────────

// customCurrentProject returns the project name matching the current working directory.
func customCurrentProject() string {
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

// newExecCmd is used by other cmd files; kept here for reuse.
func newExecCmd(name string, args ...string) *exec.Cmd {
	return exec.Command(name, args...)
}
