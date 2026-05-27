package cmd

import (
	"fmt"
	"gez/internal/custom"
	"gez/internal/git"
	"gez/internal/ui"
	"gez/internal/workspace"
	"os"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"
)

// ── ws root ───────────────────────────────────────────────────────────────────

var wsCmd = &cobra.Command{
	Use:     "ws",
	Short:   "워크스페이스 관리 (다중 프로젝트)",
	Aliases: []string{"workspace"},
	Run: func(cmd *cobra.Command, args []string) {
		runWsStatus()
	},
}

// ── ws add ────────────────────────────────────────────────────────────────────

var wsAddCmd = &cobra.Command{
	Use:   "add [경로]",
	Short: "현재 폴더(또는 지정 경로)를 워크스페이스에 등록",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		path := ""
		if len(args) > 0 {
			path = args[0]
		}

		// Determine the absolute path for validation display
		target := path
		if target == "" {
			var err error
			target, err = os.Getwd()
			if err != nil {
				ui.Fail("현재 디렉토리를 알 수 없습니다: " + err.Error())
				return
			}
		}

		if !git.IsRepoInDir(target) {
			ui.Fail(fmt.Sprintf("'%s' 은(는) git 저장소가 아닙니다", workspace.HomePath(target)))
			return
		}

		ws, err := workspace.Load()
		if err != nil {
			ui.Fail("워크스페이스 로드 실패: " + err.Error())
			return
		}

		if err := ws.Add(path); err != nil {
			ui.Fail(err.Error())
			return
		}

		proj := ws.Projects[len(ws.Projects)-1]
		ui.Success(fmt.Sprintf("'%s' 등록 완료  (%s)", proj.Name, workspace.HomePath(proj.Path)))
		fmt.Printf("  %s  %s\n", ui.Dim("이제 이렇게 사용하세요:"), ui.Cyan(fmt.Sprintf("gez -p %s <명령어>", proj.Name)))
		fmt.Println()

		// ── 커스텀 명령어 자동 감지 ──────────────────────────────────────────
		autoDetectAndRegister(proj.Name, proj.Path, true)
	},
}

// ── ws rm ─────────────────────────────────────────────────────────────────────

var wsRmCmd = &cobra.Command{
	Use:     "rm <이름>",
	Short:   "워크스페이스에서 프로젝트 제거",
	Aliases: []string{"remove", "del"},
	Args:    cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		ws, err := workspace.Load()
		if err != nil {
			ui.Fail("워크스페이스 로드 실패: " + err.Error())
			return
		}
		if err := ws.Remove(args[0]); err != nil {
			ui.Fail(err.Error())
			return
		}
		ui.Success(fmt.Sprintf("'%s' 워크스페이스에서 제거 완료", args[0]))
	},
}

// ── ws rename ─────────────────────────────────────────────────────────────────

var wsRenameCmd = &cobra.Command{
	Use:   "rename <현재이름> <새이름>",
	Short: "워크스페이스 프로젝트 이름 변경",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		ws, err := workspace.Load()
		if err != nil {
			ui.Fail("워크스페이스 로드 실패: " + err.Error())
			return
		}
		if err := ws.Rename(args[0], args[1]); err != nil {
			ui.Fail(err.Error())
			return
		}
		ui.Success(fmt.Sprintf("'%s' → '%s' 이름 변경 완료", args[0], args[1]))
	},
}

// ── ws ls ─────────────────────────────────────────────────────────────────────

var wsLsCmd = &cobra.Command{
	Use:     "ls",
	Short:   "등록된 프로젝트 목록 (빠른 조회, git 상태 없음)",
	Aliases: []string{"list"},
	Run: func(cmd *cobra.Command, args []string) {
		ws, err := workspace.Load()
		if err != nil {
			ui.Fail("워크스페이스 로드 실패: " + err.Error())
			return
		}
		if len(ws.Projects) == 0 {
			ui.Info("등록된 프로젝트가 없습니다")
			fmt.Printf("  %s\n", ui.Dim("gez ws add  →  현재 폴더 등록"))
			return
		}

		fmt.Println()
		nameW := 8
		for _, p := range ws.Projects {
			if len(p.Name) > nameW {
				nameW = len(p.Name)
			}
		}
		for i, p := range ws.Projects {
			fmt.Printf("  %s%-*s  %s\n",
				ui.Dim(fmt.Sprintf("%2d. ", i+1)),
				nameW+2, ui.BoldCyan(p.Name),
				ui.Dim(workspace.HomePath(p.Path)))
		}
		fmt.Println()
		fmt.Printf("  %s  %s\n", ui.Dim("사용법:"), ui.Cyan("gez -p <이름> <명령어>"))
		fmt.Println()
	},
}

// ── ws status ─────────────────────────────────────────────────────────────────

var wsStatusCmd = &cobra.Command{
	Use:     "status",
	Short:   "모든 프로젝트의 git 상태 보기",
	Aliases: []string{"st"},
	Run: func(cmd *cobra.Command, args []string) {
		runWsStatus()
	},
}

func runWsStatus() {
	ws, err := workspace.Load()
	if err != nil {
		ui.Fail("워크스페이스 로드 실패: " + err.Error())
		return
	}
	if len(ws.Projects) == 0 {
		ui.Info("등록된 프로젝트가 없습니다")
		fmt.Printf("  %s\n", ui.Dim("gez ws add [경로]  →  프로젝트 등록"))
		return
	}

	fmt.Println()
	fmt.Printf("  %s\n", ui.Bold(fmt.Sprintf("Workspace Status  —  %d개 프로젝트", len(ws.Projects))))
	fmt.Println()

	nameW := 8
	pathW := 20
	for _, p := range ws.Projects {
		if len(p.Name) > nameW {
			nameW = len(p.Name)
		}
		hp := workspace.HomePath(p.Path)
		if len(hp) > pathW {
			pathW = len(hp)
		}
	}
	if pathW > 42 {
		pathW = 42
	}

	sep := strings.Repeat("─", nameW+pathW+32)
	fmt.Println(ui.Dim(sep))

	for _, p := range ws.Projects {
		hp := workspace.HomePath(p.Path)

		if !git.IsRepoInDir(p.Path) {
			fmt.Printf("  %-*s  %-*s  %s\n",
				nameW, p.Name,
				pathW, truncate(hp, pathW),
				ui.Red("⚠ 경로 없음"))
			continue
		}

		branch := git.CurrentBranchInDir(p.Path)
		ahead, behind := git.AheadBehindInDir(p.Path)
		lines := git.StatusShortInDir(p.Path)

		staged, unstaged, untracked := 0, 0, 0
		for _, l := range lines {
			if len(l) < 3 {
				continue
			}
			x, y := l[0], l[1]
			if x == '?' && y == '?' {
				untracked++
			} else {
				if x != ' ' && x != '?' {
					staged++
				}
				if y != ' ' && y != '?' {
					unstaged++
				}
			}
		}

		syncPart := ""
		if ahead != "" && ahead != "0" {
			syncPart += ui.Green("↑" + ahead)
		}
		if behind != "" && behind != "0" {
			if syncPart != "" {
				syncPart += " "
			}
			syncPart += ui.Red("↓" + behind)
		}

		var parts []string
		if staged > 0 {
			parts = append(parts, ui.Green(fmt.Sprintf("%d staged", staged)))
		}
		if unstaged > 0 {
			parts = append(parts, ui.Yellow(fmt.Sprintf("%d unstaged", unstaged)))
		}
		if untracked > 0 {
			parts = append(parts, ui.Blue(fmt.Sprintf("%d untracked", untracked)))
		}
		statusPart := ui.Dim("깨끗")
		if len(parts) > 0 {
			statusPart = strings.Join(parts, "  ")
		}

		branchStr := ui.BoldCyan(branch)
		if syncPart != "" {
			branchStr += " " + syncPart
		}

		fmt.Printf("  %-*s  %-*s  [%s]  %s\n",
			nameW, p.Name,
			pathW, truncate(hp, pathW),
			branchStr,
			statusPart)
	}

	fmt.Println(ui.Dim(sep))
	fmt.Println()
	fmt.Printf("  %s  %s\n", ui.Dim("Tip:"), ui.Cyan("gez -p <이름> <명령어>"))
	fmt.Println()
}

// ── ws pull ───────────────────────────────────────────────────────────────────

var wsPullCmd = &cobra.Command{
	Use:   "pull",
	Short: "모든 워크스페이스 프로젝트 풀",
	Run: func(cmd *cobra.Command, args []string) {
		runWsBulk("pull", "pull")
	},
}

// ── ws fetch ──────────────────────────────────────────────────────────────────

var wsFetchCmd = &cobra.Command{
	Use:   "fetch",
	Short: "모든 워크스페이스 프로젝트 fetch",
	Run: func(cmd *cobra.Command, args []string) {
		runWsBulk("fetch", "fetch", "--all", "--prune")
	},
}

// ── ws sync ───────────────────────────────────────────────────────────────────

var wsSyncCmd = &cobra.Command{
	Use:   "sync",
	Short: "모든 워크스페이스 프로젝트 fetch + pull",
	Run: func(cmd *cobra.Command, args []string) {
		ws, err := workspace.Load()
		if err != nil {
			ui.Fail("워크스페이스 로드 실패: " + err.Error())
			return
		}
		if len(ws.Projects) == 0 {
			ui.Info("등록된 프로젝트가 없습니다")
			return
		}

		fmt.Println()
		fmt.Printf("  %s\n\n", ui.Bold(fmt.Sprintf("Workspace Sync  —  %d개 프로젝트", len(ws.Projects))))

		for _, p := range ws.Projects {
			fmt.Println(ui.Dim(strings.Repeat("─", 48)))
			fmt.Printf("  %s  %s\n", ui.BoldCyan(p.Name), ui.Dim(workspace.HomePath(p.Path)))
			fmt.Println()

			if !git.IsRepoInDir(p.Path) {
				ui.Warn("git 저장소가 아닙니다 — 건너뜀")
				continue
			}

			fmt.Printf("  %s fetch --all --prune\n", ui.Dim("1."))
			if err := git.RunLiveInDir(p.Path, "fetch", "--all", "--prune"); err != nil {
				ui.Warn("fetch 실패 — 건너뜀")
				continue
			}
			fmt.Printf("\n  %s pull\n", ui.Dim("2."))
			if err := git.RunLiveInDir(p.Path, "pull"); err != nil {
				ui.Warn("pull 실패 (충돌 또는 upstream 없음)")
				continue
			}
			fmt.Println()
			ui.Success("동기화 완료!")
		}

		fmt.Println()
		fmt.Println(ui.Dim(strings.Repeat("─", 48)))
		ui.Success(fmt.Sprintf("전체 워크스페이스 동기화 완료 (%d개 프로젝트)", len(ws.Projects)))
		fmt.Println()
	},
}

// ── ws interactive select ─────────────────────────────────────────────────────

var wsGoCmd = &cobra.Command{
	Use:   "go",
	Short: "프로젝트 선택 후 대화형 명령 실행",
	Run: func(cmd *cobra.Command, args []string) {
		ws, err := workspace.Load()
		if err != nil {
			ui.Fail("워크스페이스 로드 실패: " + err.Error())
			return
		}
		if len(ws.Projects) == 0 {
			ui.Info("등록된 프로젝트가 없습니다")
			return
		}

		options := make([]string, len(ws.Projects))
		for i, p := range ws.Projects {
			options[i] = fmt.Sprintf("%-16s %s", p.Name, ui.Dim(workspace.HomePath(p.Path)))
		}

		var selected string
		if err := survey.AskOne(&survey.Select{
			Message: "프로젝트 선택:",
			Options: options,
		}, &selected); err != nil {
			return
		}

		// Extract name (first field)
		name := strings.Fields(selected)[0]
		proj := ws.Find(name)
		if proj == nil {
			return
		}

		if err := os.Chdir(proj.Path); err != nil {
			ui.Fail("프로젝트 폴더 이동 실패: " + err.Error())
			return
		}

		fmt.Printf("\n  %s %s  %s\n\n",
			ui.Dim("프로젝트:"),
			ui.BoldCyan(proj.Name),
			ui.Dim(workspace.HomePath(proj.Path)))

		showDashboard()
	},
}

// ── runWsBulk helper ──────────────────────────────────────────────────────────

func runWsBulk(label string, gitArgs ...string) {
	ws, err := workspace.Load()
	if err != nil {
		ui.Fail("워크스페이스 로드 실패: " + err.Error())
		return
	}
	if len(ws.Projects) == 0 {
		ui.Info("등록된 프로젝트가 없습니다")
		return
	}

	fmt.Println()
	fmt.Printf("  %s\n\n", ui.Bold(fmt.Sprintf("Workspace %s  —  %d개 프로젝트", strings.ToUpper(label), len(ws.Projects))))

	ok, fail := 0, 0
	for _, p := range ws.Projects {
		fmt.Println(ui.Dim(strings.Repeat("─", 48)))
		fmt.Printf("  %s  %s\n\n", ui.BoldCyan(p.Name), ui.Dim(workspace.HomePath(p.Path)))

		if !git.IsRepoInDir(p.Path) {
			ui.Warn("git 저장소가 아닙니다 — 건너뜀")
			fail++
			continue
		}

		if err := git.RunLiveInDir(p.Path, gitArgs...); err != nil {
			ui.Warn(fmt.Sprintf("%s 실패", label))
			fail++
			continue
		}
		ui.Success(fmt.Sprintf("%s 완료!", label))
		ok++
	}

	fmt.Println()
	fmt.Println(ui.Dim(strings.Repeat("─", 48)))
	if fail == 0 {
		ui.Success(fmt.Sprintf("전체 완료 (%d개)", ok))
	} else {
		ui.Warn(fmt.Sprintf("완료 %d개, 실패 %d개", ok, fail))
	}
	fmt.Println()
}

// ── ws foreach ────────────────────────────────────────────────────────────────

var wsForeachCmd = &cobra.Command{
	Use:   "foreach <명령어>",
	Short: "모든 워크스페이스 프로젝트에서 명령어 실행",
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		command := strings.Join(args, " ")
		runWsForeach(command)
	},
}

func runWsForeach(command string) {
	ws, err := workspace.Load()
	if err != nil {
		ui.Fail("워크스페이스 로드 실패: " + err.Error())
		return
	}
	if len(ws.Projects) == 0 {
		ui.Info("등록된 프로젝트가 없습니다")
		return
	}

	fmt.Println()
	fmt.Printf("  %s\n", ui.Bold(fmt.Sprintf("Workspace Foreach  —  %d개 프로젝트", len(ws.Projects))))
	fmt.Printf("  %s  %s\n\n", ui.Dim("명령어:"), ui.Cyan(command))

	ok, fail := 0, 0
	for _, p := range ws.Projects {
		fmt.Println(ui.Dim(strings.Repeat("─", 52)))
		fmt.Printf("  %s  %s\n\n", ui.BoldCyan(p.Name), ui.Dim(workspace.HomePath(p.Path)))

		if !git.IsRepoInDir(p.Path) {
			ui.Warn("git 저장소가 아닙니다 — 건너뜀")
			fail++
			continue
		}

		// Execute the command with git -C
		if err := git.RunLiveInDir(p.Path, strings.Fields(command)...); err != nil {
			// Try as shell command if git command fails
			ui.Warn(fmt.Sprintf("명령 실패: %s", err.Error()))
			fail++
			continue
		}
		ok++
		fmt.Println()
	}

	fmt.Println(ui.Dim(strings.Repeat("─", 52)))
	if fail == 0 {
		ui.Success(fmt.Sprintf("전체 완료 (%d개)", ok))
	} else {
		ui.Warn(fmt.Sprintf("완료 %d개, 실패 %d개", ok, fail))
	}
	fmt.Println()
}

// ── autoDetectAndRegister ─────────────────────────────────────────────────────

// autoDetectAndRegister scans dir for build scripts and saves detected commands.
// If announce is true it prints a summary line.
func autoDetectAndRegister(projName, dir string, announce bool) {
	result := custom.DetectCommands(dir)
	if len(result.Commands) == 0 {
		if announce {
			fmt.Printf("  %s  감지된 명령어 없음 (나중에 gez custom add 로 추가)\n\n", ui.Dim("자동 감지:"))
		}
		return
	}

	cfg := custom.Load()
	added := 0
	for _, c := range result.Commands {
		before := len(cfg.ForProject(projName).Commands)
		cfg.AddCommand(projName, c)
		after := len(cfg.ForProject(projName).Commands)
		if after > before {
			added++
		}
	}
	if err := custom.Save(cfg); err != nil {
		if announce {
			ui.Warn("커스텀 명령어 저장 실패: " + err.Error())
		}
		return
	}

	if announce {
		fmt.Printf("  %s  %s개 명령어 자동 등록  (%s)\n",
			ui.Dim("자동 감지:"),
			ui.BoldCyan(fmt.Sprintf("%d", len(result.Commands))),
			ui.Dim(strings.Join(result.Sources, ", ")))
		fmt.Printf("  %s  %s\n\n",
			ui.Dim("TUI 확인:"),
			ui.Cyan("gez ui  →  [3] Commands 탭"))
	}
	_ = added
}

// ── init ──────────────────────────────────────────────────────────────────────

func init() {
	wsCmd.AddCommand(wsAddCmd)
	wsCmd.AddCommand(wsRmCmd)
	wsCmd.AddCommand(wsRenameCmd)
	wsCmd.AddCommand(wsLsCmd)
	wsCmd.AddCommand(wsStatusCmd)
	wsCmd.AddCommand(wsPullCmd)
	wsCmd.AddCommand(wsFetchCmd)
	wsCmd.AddCommand(wsSyncCmd)
	wsCmd.AddCommand(wsGoCmd)
	wsCmd.AddCommand(wsForeachCmd)
	rootCmd.AddCommand(wsCmd)
}
