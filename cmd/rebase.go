package cmd

import (
	"fmt"
	"gez/internal/git"
	"gez/internal/ui"
	"os"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"
)

var rebaseCmd = &cobra.Command{
	Use:   "rebase [브랜치]",
	Short: "브랜치 리베이스 / interactive rebase",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if !git.IsRepo() {
			ui.Fail("git 저장소가 아닙니다")
			os.Exit(1)
		}
		// rebase 진행 중이면 제어 메뉴 먼저
		if git.IsRebaseInProgress() {
			runRebaseInProgress()
			return
		}
		if len(args) > 0 {
			doRebase(args[0])
		} else {
			runRebaseMenu()
		}
	},
}

func init() {
	rootCmd.AddCommand(rebaseCmd)
}

// ── 진행 중 처리 ──────────────────────────────────────────────────────────────

func runRebaseInProgress() {
	ui.Warn("rebase가 진행 중입니다")
	fmt.Println()

	var action string
	if err := survey.AskOne(&survey.Select{
		Message: "작업 선택:",
		Options: []string{
			"계속 진행  (rebase --continue)",
			"이 커밋 건너뛰기  (rebase --skip)",
			"rebase 중단  (rebase --abort)",
		},
	}, &action); err != nil {
		return
	}

	fmt.Println()
	switch {
	case strings.HasPrefix(action, "계속"):
		if err := git.RunLive("rebase", "--continue"); err != nil {
			ui.Warn("계속 실패 — 충돌 파일을 모두 해결하고 스테이징했는지 확인하세요")
		} else {
			ui.Success("rebase 계속 완료!")
		}
	case strings.HasPrefix(action, "이 커밋"):
		if err := git.RunLive("rebase", "--skip"); err != nil {
			ui.Fail("--skip 실패")
		} else {
			ui.Success("커밋을 건너뛰었습니다")
		}
	case strings.HasPrefix(action, "rebase 중단"):
		if err := git.RunLive("rebase", "--abort"); err != nil {
			ui.Fail("--abort 실패")
		} else {
			ui.Success("rebase 중단 — 원래 상태로 복구됐습니다")
		}
	}
	fmt.Println()
}

// ── 대화형 메뉴 ───────────────────────────────────────────────────────────────

func runRebaseMenu() {
	current := git.CurrentBranch()
	fmt.Println()
	fmt.Printf("  %s  %s\n\n", ui.Bold("현재 브랜치:"), ui.BoldCyan(current))

	var mode string
	if err := survey.AskOne(&survey.Select{
		Message: "rebase 방식:",
		Options: []string{
			"브랜치 선택 rebase  — 다른 브랜치 위로 올라타기",
			"Interactive rebase  — 최근 N개 커밋 squash·reword·drop",
			"취소",
		},
	}, &mode); err != nil || mode == "취소" {
		return
	}

	if strings.HasPrefix(mode, "Interactive") {
		runInteractiveRebase()
		return
	}

	// 브랜치 선택
	locals := git.LocalBranches()
	var options []string
	for _, b := range locals {
		if b != current {
			options = append(options, b)
		}
	}
	for _, r := range git.RemoteBranches() {
		short := strings.TrimPrefix(r, "origin/")
		if short != current && !contains(locals, short) {
			options = append(options, fmt.Sprintf("%s  %s", r, ui.Dim("(remote)")))
		}
	}
	if len(options) == 0 {
		ui.Info("rebase 대상 브랜치가 없습니다")
		return
	}

	var selected string
	if err := survey.AskOne(&survey.Select{
		Message: "rebase 베이스 브랜치 (이 브랜치 위로 올라탑니다):",
		Options: options,
	}, &selected); err != nil {
		return
	}

	doRebase(strings.Fields(selected)[0])
}

// ── Interactive rebase ────────────────────────────────────────────────────────

func runInteractiveRebase() {
	// 최근 커밋 미리보기 + 몇 개 편집할지 선택
	recent := git.RecentCommits(10)
	if len(recent) == 0 {
		ui.Info("편집할 커밋이 없습니다")
		return
	}

	fmt.Println()
	fmt.Printf("  %s\n", ui.Bold("최근 커밋:"))
	for i, c := range recent {
		fmt.Printf("    %s  %s\n", ui.Dim(fmt.Sprintf("[%d]", i+1)), c)
	}
	fmt.Println()

	var countStr string
	if err := survey.AskOne(&survey.Input{
		Message: "몇 개의 커밋을 편집할까요? (HEAD~N)",
		Default: "3",
	}, &countStr); err != nil {
		return
	}
	n := strings.TrimSpace(countStr)
	if n == "" || n == "0" {
		ui.Warn("유효한 숫자를 입력하세요")
		return
	}

	fmt.Println()
	ui.Info("에디터가 열립니다 — pick·squash·fixup·reword·drop 으로 편집 후 저장하세요")
	fmt.Println()

	if err := git.RunLive("rebase", "-i", "HEAD~"+n); err != nil {
		fmt.Println()
		if git.IsRebaseInProgress() {
			ui.Warn("충돌 발생 — gez rebase 로 계속/건너뛰기/중단 선택")
		} else {
			ui.Fail("interactive rebase 실패")
		}
	} else {
		fmt.Println()
		ui.Success("interactive rebase 완료!")
	}
	fmt.Println()
}

// ── 실행 ─────────────────────────────────────────────────────────────────────

func doRebase(target string) {
	current := git.CurrentBranch()

	// 미리보기: 이동될 커밋
	commits := git.CommitsBetween(target, current)
	fmt.Println()
	if len(commits) == 0 {
		ui.Info(fmt.Sprintf("'%s' 대비 새로운 커밋이 없습니다 — rebase 불필요", target))
		return
	}
	fmt.Printf("  %s  '%s' 위로 옮겨질 커밋:\n", ui.Bold("Preview:"), ui.Cyan(target))
	for _, c := range commits {
		fmt.Printf("    %s  %s\n", ui.Yellow("·"), c)
	}
	fmt.Println()

	var ok bool
	if err := survey.AskOne(&survey.Confirm{
		Message: fmt.Sprintf("'%s' 위로 '%s' 를 rebase할까요?", target, current),
		Default: true,
	}, &ok); err != nil || !ok {
		ui.Warn("취소되었습니다")
		return
	}

	fmt.Println()
	if err := git.RunLive("rebase", target); err != nil {
		fmt.Println()
		if git.IsRebaseInProgress() {
			ui.Warn("충돌 발생 — 충돌 파일 수정 후 gez rebase 로 제어하세요")
			fmt.Printf("    %s  스테이징 후 계속\n", ui.Cyan("gez rebase  →  계속 진행"))
			fmt.Printf("    %s  취소\n", ui.Cyan("gez rebase  →  rebase 중단"))
		} else {
			ui.Fail("rebase 실패")
		}
	} else {
		fmt.Println()
		ui.Success(fmt.Sprintf("'%s' 위로 rebase 완료!", target))
	}
	fmt.Println()
}
