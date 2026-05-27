package cmd

import (
	"fmt"
	"gez/internal/git"
	"gez/internal/ui"
	"gez/internal/workspace"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

var projectFlag string

var rootCmd = &cobra.Command{
	Use:   "gez",
	Short: "Git Easy — git 작업을 간편하게",
	Long:  `gez — 커밋·푸시·브랜치·스태시·태그까지 대화형으로 빠르게 처리합니다.`,

	// PersistentPreRunE: 모든 하위 명령 실행 전 -p 플래그 처리
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// ws 관리 명령어에는 프로젝트 전환을 적용하지 않음
		if isWorkspaceCmd(cmd) {
			return nil
		}
		if projectFlag != "" {
			ws, err := workspace.Load()
			if err != nil {
				return fmt.Errorf("워크스페이스 로드 실패: %w", err)
			}
			proj := ws.Find(projectFlag)
			if proj == nil {
				return fmt.Errorf("프로젝트 '%s'를 찾을 수 없습니다\n  목록 확인: gez ws ls", projectFlag)
			}
			if err := os.Chdir(proj.Path); err != nil {
				return fmt.Errorf("프로젝트 폴더 이동 실패 (%s): %w", proj.Path, err)
			}
			fmt.Printf("\n  %s %s  %s\n",
				ui.Dim("프로젝트:"),
				ui.BoldCyan(proj.Name),
				ui.Dim(workspace.HomePath(proj.Path)))
		}
		return nil
	},

	Run: func(cmd *cobra.Command, args []string) {
		if !git.IsRepo() {
			showWorkspaceOverview()
			return
		}
		showDashboard()
	},
}

// isWorkspaceCmd returns true if the command is a ws subcommand.
func isWorkspaceCmd(cmd *cobra.Command) bool {
	path := cmd.CommandPath()
	return strings.Contains(path, " ws") || cmd.Name() == "ws"
}

// Execute is the entry point called from main.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVarP(
		&projectFlag, "project", "p", "",
		"워크스페이스에 등록된 프로젝트 이름으로 해당 프로젝트에서 명령 실행",
	)
}

// ── Dashboard (current repo) ──────────────────────────────────────────────────

func showDashboard() {
	branch := git.CurrentBranch()

	ahead, behind := git.AheadBehind()
	syncInfo := ""
	if ahead != "" && ahead != "0" {
		syncInfo += ui.Green(fmt.Sprintf("↑%s", ahead))
	}
	if behind != "" && behind != "0" {
		if syncInfo != "" {
			syncInfo += " "
		}
		syncInfo += ui.Red(fmt.Sprintf("↓%s", behind))
	}

	branchDisplay := ui.BoldCyan(branch)
	if syncInfo != "" {
		branchDisplay += "  " + syncInfo
	}

	fmt.Println()
	fmt.Printf("  %s  %s\n", ui.Bold("Branch:"), branchDisplay)
	fmt.Println()

	lines := git.StatusShort()
	if len(lines) == 0 {
		ui.Success("워킹 트리가 깨끗합니다")
	} else {
		fmt.Printf("  %s\n", ui.Bold("변경사항:"))
		for _, l := range lines {
			if len(l) < 3 {
				continue
			}
			fmt.Printf("    %s  %s\n", ui.ColorXY(l[:2]), l[3:])
		}
	}

	fmt.Println()
	fmt.Println(ui.Dim(strings.Repeat("─", 52)))
	fmt.Println(ui.Bold("  명령어 목록"))
	fmt.Println(ui.Dim(strings.Repeat("─", 52)))
	cmds := [][2]string{
		{"gez s   (status)    ", "현재 상태 자세히 보기"},
		{"gez c   (commit)    ", "커밋 마법사 (스테이징→메시지→push)"},
		{"gez p   (push)      ", "원격에 푸시  [-f: force-with-lease]"},
		{"gez pull            ", "원격에서 풀"},
		{"gez f   (fetch)     ", "원격 정보 가져오기"},
		{"gez sync            ", "fetch + pull (원격 동기화)"},
		{"gez l   (log)       ", "커밋 그래프 로그"},
		{"gez b   (branch)    ", "브랜치 관리 (전환·생성·삭제)"},
		{"gez d   (diff)      ", "변경사항 diff 보기"},
		{"gez stash           ", "스태시 관리 (push·pop·apply·drop)"},
		{"gez reset           ", "언스테이징 / 커밋 되돌리기"},
		{"gez merge           ", "브랜치 병합"},
		{"gez tag             ", "태그 관리 (생성·삭제·push)"},
		{"gez remote          ", "원격 저장소 관리"},
		{"gez init [경로]     ", "새 git 저장소 초기화"},
		{"gez clone <url>     ", "저장소 클론"},
		{"gez ws              ", "워크스페이스 — 다중 프로젝트 관리"},
	}
	for _, pair := range cmds {
		fmt.Printf("  %s  %s\n", ui.Cyan(pair[0]), ui.Dim(pair[1]))
	}
	fmt.Println()
	fmt.Println(ui.Dim(strings.Repeat("─", 52)))
	fmt.Printf("  %s  %s\n", ui.Dim("Tip:"), ui.Dim("gez -p <프로젝트명> <명령어>  →  다른 폴더에 있는 프로젝트에서 실행"))
	fmt.Println()
}

// ── Workspace overview (shown when not in a git repo) ─────────────────────────

func showWorkspaceOverview() {
	ws, err := workspace.Load()
	if err != nil || len(ws.Projects) == 0 {
		ui.Fail("git 저장소가 아닙니다")
		fmt.Println()
		fmt.Printf("  %s\n", ui.Dim("git 저장소 안에서 실행하거나, 아래 명령어로 프로젝트를 등록하세요:"))
		fmt.Printf("    %s\n", ui.Cyan("gez ws add [경로]"))
		fmt.Println()
		os.Exit(1)
	}

	fmt.Println()
	fmt.Printf("  %s\n", ui.Bold(fmt.Sprintf("Workspace  —  %d개 프로젝트", len(ws.Projects))))
	fmt.Println()

	// Compute column widths (raw strings, no ANSI)
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
	if pathW > 40 {
		pathW = 40
	}

	fmt.Println(ui.Dim(strings.Repeat("─", nameW+pathW+30)))

	for _, p := range ws.Projects {
		hp := workspace.HomePath(p.Path)

		if !git.IsRepoInDir(p.Path) {
			fmt.Printf("  %-*s  %-*s  %s\n",
				nameW, p.Name,
				pathW, truncate(hp, pathW),
				ui.Red("⚠ 경로 없음 또는 git 저장소 아님"))
			continue
		}

		branch := git.CurrentBranchInDir(p.Path)
		ahead, behind := git.AheadBehindInDir(p.Path)
		lines := git.StatusShortInDir(p.Path)

		changed := 0
		for _, l := range lines {
			if len(l) >= 3 {
				changed++
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

		statusPart := ui.Dim("깨끗")
		if changed > 0 {
			statusPart = ui.Yellow(fmt.Sprintf("%d 변경", changed))
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

	fmt.Println(ui.Dim(strings.Repeat("─", nameW+pathW+30)))
	fmt.Println()
	fmt.Printf("  %s  %s\n", ui.Cyan(fmt.Sprintf("%-22s", "gez -p <이름> <명령어>")), ui.Dim("특정 프로젝트에서 명령 실행"))
	fmt.Printf("  %s  %s\n", ui.Cyan(fmt.Sprintf("%-22s", "gez ws add [경로]")), ui.Dim("현재 폴더(또는 경로)를 워크스페이스에 등록"))
	fmt.Printf("  %s  %s\n", ui.Cyan(fmt.Sprintf("%-22s", "gez ws pull")), ui.Dim("전체 프로젝트 풀"))
	fmt.Printf("  %s  %s\n", ui.Cyan(fmt.Sprintf("%-22s", "gez ws sync")), ui.Dim("전체 프로젝트 fetch + pull"))
	fmt.Printf("  %s  %s\n", ui.Cyan(fmt.Sprintf("%-22s", "gez ws status")), ui.Dim("전체 프로젝트 상태 (이 화면)"))
	fmt.Println()
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	if max <= 3 {
		return s[:max]
	}
	return "..." + s[len(s)-(max-3):]
}
