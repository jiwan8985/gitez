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

// appVersion is set at startup via SetVersion (injected from main.go ldflags).
var appVersion = "dev"

// SetVersion lets main.go inject the build-time version string.
func SetVersion(v string) {
	appVersion = v
	rootCmd.Version = v
}

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
	sep := ui.Dim(strings.Repeat("─", 52))
	printSection := func(title string, rows [][2]string) {
		fmt.Println(sep)
		fmt.Printf("  %s\n", ui.Bold(title))
		fmt.Println(sep)
		for _, pair := range rows {
			fmt.Printf("  %s  %s\n", ui.Cyan(pair[0]), ui.Dim(pair[1]))
		}
		fmt.Println()
	}

	printSection("기본 워크플로우", [][2]string{
		{"gez s   (status)    ", "현재 상태 자세히 보기"},
		{"gez c   (commit)    ", "커밋 마법사 (스테이징→메시지→push)"},
		{"gez p   (push)      ", "원격에 푸시  [-f: force-with-lease]"},
		{"gez pull            ", "원격에서 풀"},
		{"gez sync            ", "fetch + pull (원격 동기화)"},
		{"gez f   (fetch)     ", "원격 정보 가져오기"},
		{"gez l   (log)       ", "커밋 그래프 로그"},
		{"gez d   (diff)      ", "변경사항 diff 보기"},
	})

	printSection("브랜치 & 히스토리 관리", [][2]string{
		{"gez b   (branch)    ", "브랜치 전환·생성·삭제"},
		{"gez merge           ", "브랜치 병합"},
		{"gez rebase          ", "브랜치 리베이스 / interactive -i"},
		{"gez cp (cherry-pick)", "다른 브랜치 커밋 가져오기"},
		{"gez revert          ", "커밋 되돌리기 (히스토리 유지)"},
		{"gez reset           ", "언스테이징 / soft·mixed·hard reset"},
	})

	printSection("복구 & 정리", [][2]string{
		{"gez stash           ", "스태시 push·pop·apply·drop (diff 미리보기 포함)"},
		{"gez reflog          ", "reflog 조회 + 사라진 커밋 복구"},
		{"gez blame [파일]    ", "줄별 작성자·커밋 보기"},
		{"gez clean           ", "untracked 파일·디렉토리 정리"},
		{"gez squash [n]      ", "최근 N개 커밋을 하나로 합치기"},
		{"gez amend           ", "마지막 커밋 수정 (메시지·파일 추가)"},
	})

	printSection("저장소 & 원격 관리", [][2]string{
		{"gez tag             ", "태그 생성·삭제·push"},
		{"gez remote          ", "원격 저장소 관리"},
		{"gez init [경로]     ", "새 git 저장소 초기화"},
		{"gez clone <url>     ", "저장소 클론"},
		{"gez worktree (wt)   ", "워크트리 add·list·remove·prune"},
		{"gez submodule (sub) ", "서브모듈 add·update·sync·foreach"},
		{"gez bisect          ", "이진 탐색으로 버그 도입 커밋 찾기"},
		{"gez pr              ", "PR/MR 생성 URL을 브라우저로 열기"},
		{"gez hook            ", "Git hooks 관리 (활성화·비활성화·프리셋)"},
		{"gez config          ", "Git + gez 설정 조회/수정"},
		{"gez archive         ", "저장소를 zip·tar.gz 파일로 내보내기"},
		{"gez patch           ", "패치 파일 생성·적용 (format-patch·apply)"},
		{"gez sparse          ", "Sparse checkout 관리 (모노레포)"},
	})

	printSection("검색 & 분석", [][2]string{
		{"gez search          ", "커밋 메시지·코드 내용·파일명 검색"},
		{"gez show [hash]     ", "커밋 상세 보기 (대화형 선택 가능)"},
		{"gez stats           ", "저장소 통계 (기여자·활동·파일 분석)"},
		{"gez file [경로]     ", "파일별 히스토리·blame·복원 통합 메뉴"},
		{"gez log -i          ", "커밋 선택 후 show·cherry-pick·reset"},
	})

	printSection("커밋 관리", [][2]string{
		{"gez squash [n]      ", "최근 N개 커밋을 하나로 합치기"},
		{"gez amend           ", "마지막 커밋 수정 (메시지·파일 추가)"},
		{"gez fixup           ", "특정 커밋의 fixup 커밋 생성 + autosquash"},
		{"gez undo            ", "마지막 git 작업 취소 (reflog 기반)"},
		{"gez restore         ", "파일을 HEAD·특정 커밋으로 복원"},
		{"gez changelog       ", "Conventional Commits 기반 CHANGELOG 생성"},
	})

	printSection("환경 설정", [][2]string{
		{"gez ignore          ", ".gitignore 관리 (패턴 추가·템플릿 15종)"},
		{"gez alias           ", "Git alias 관리 (추가·삭제·프리셋)"},
		{"gez doctor          ", "Git 환경 진단 (설정·SSH·버전·연결)"},
		{"gez completion-install", "쉘 자동완성 설치 (bash·zsh·fish·PowerShell)"},
	})

	printSection("TUI 모드", [][2]string{
		{"gez ui (tui)        ", "전체화면 TUI — stage·diff·log 한눈에"},
		{"                    ", "  h:hunk staging  d:diff  c:커밋  p:push  P:pull  b:브랜치  l:로그"},
		{"gez log -i          ", "커밋 선택 후 show·cherry-pick·reset"},
	})

	printSection("브랜치 전략 (Git Flow / GitHub Flow / Trunk)", [][2]string{
		{"gez flow init                ", "전략 초기화 (최초 1회)"},
		{"gez flow                     ", "전략 현황 + 다음 명령 힌트"},
		{"gez flow feature start <이름>", "feature 브랜치 시작"},
		{"gez flow feature finish      ", "feature 완료 → develop 머지"},
		{"gez flow release start <버전>", "release 브랜치 시작 (Git Flow)"},
		{"gez flow release finish      ", "release 완료 → main+develop+태그"},
		{"gez flow hotfix start <이름> ", "hotfix 시작 (프로덕션 긴급 수정)"},
		{"gez flow hotfix finish       ", "hotfix 완료 → main+develop+태그"},
	})

	printSection("워크스페이스 (다중 프로젝트)", [][2]string{
		{"gez ws              ", "전체 프로젝트 상태"},
		{"gez ws add [경로]   ", "프로젝트 등록"},
		{"gez ws pull/sync    ", "전체 프로젝트 pull/sync"},
		{"gez ws foreach <cmd>", "모든 프로젝트에서 git 명령 실행"},
		{"gez -p <이름> <cmd> ", "특정 프로젝트에서 명령 실행"},
	})

	fmt.Println(ui.Dim(strings.Repeat("─", 52)))
	fmt.Printf("  %s  %s\n", ui.Dim("Tip:"), ui.Dim("gez -p <프로젝트명> <명령어>  →  다른 폴더 프로젝트에서 바로 실행"))
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
