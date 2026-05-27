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

var revertCmd = &cobra.Command{
	Use:   "revert [해시]",
	Short: "커밋을 안전하게 되돌리기 (히스토리 유지)",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if !git.IsRepo() {
			ui.Fail("git 저장소가 아닙니다")
			os.Exit(1)
		}
		// revert 진행 중이면 제어 메뉴
		if git.IsRevertInProgress() {
			runRevertInProgress()
			return
		}
		if len(args) > 0 {
			doRevert(args[0], false)
		} else {
			runRevertInteractive()
		}
	},
}

func init() {
	rootCmd.AddCommand(revertCmd)
}

// ── 진행 중 처리 ──────────────────────────────────────────────────────────────

func runRevertInProgress() {
	ui.Warn("revert가 진행 중입니다")
	fmt.Println()

	var action string
	if err := survey.AskOne(&survey.Select{
		Message: "작업 선택:",
		Options: []string{
			"계속 진행  (revert --continue)",
			"revert 중단  (revert --abort)",
		},
	}, &action); err != nil {
		return
	}

	fmt.Println()
	switch {
	case strings.HasPrefix(action, "계속"):
		if err := git.RunLive("revert", "--continue"); err != nil {
			ui.Warn("계속 실패 — 충돌 파일을 해결하고 스테이징했는지 확인하세요")
		} else {
			ui.Success("revert 완료!")
		}
	case strings.HasPrefix(action, "revert 중단"):
		if err := git.RunLive("revert", "--abort"); err != nil {
			ui.Fail("--abort 실패")
		} else {
			ui.Success("revert 중단 — 원래 상태로 복구됐습니다")
		}
	}
	fmt.Println()
}

// ── 대화형: 커밋 선택 ─────────────────────────────────────────────────────────

func runRevertInteractive() {
	commits := git.RecentCommits(20)
	if len(commits) == 0 {
		ui.Info("되돌릴 커밋이 없습니다")
		return
	}

	fmt.Println()
	fmt.Printf("  %s\n\n", ui.Bold("revert — 커밋을 새 커밋으로 취소합니다 (히스토리 유지)"))

	// 단일 선택 (revert는 보통 1개씩)
	var selected string
	if err := survey.AskOne(&survey.Select{
		Message: "되돌릴 커밋 선택:",
		Options: commits,
	}, &selected); err != nil {
		return
	}

	hash := strings.Fields(selected)[0]

	// 커밋 상세 미리보기
	detail, _ := git.Run("show", "--stat", "--oneline", hash)
	fmt.Println()
	fmt.Printf("  %s\n", ui.Bold("되돌릴 커밋:"))
	for _, l := range strings.Split(detail, "\n") {
		fmt.Printf("    %s\n", ui.Dim(l))
	}
	fmt.Println()

	// 커밋할지 여부
	var noCommit bool
	if err := survey.AskOne(&survey.Confirm{
		Message: "바로 커밋하지 않고 워킹 트리만 되돌릴까요? (--no-commit)",
		Default: false,
	}, &noCommit); err != nil {
		return
	}

	doRevert(hash, noCommit)
}

// ── 실행 ─────────────────────────────────────────────────────────────────────

func doRevert(hash string, noCommit bool) {
	fmt.Println()

	args := []string{"revert"}
	if noCommit {
		args = append(args, "--no-commit")
	}
	args = append(args, hash)

	if err := git.RunLive(args...); err != nil {
		fmt.Println()
		if git.IsRevertInProgress() {
			ui.Warn("충돌 발생 — 충돌 파일 수정 후:")
			fmt.Printf("    %s  스테이징 후 계속\n", ui.Cyan("gez revert  →  계속 진행"))
			fmt.Printf("    %s  취소\n", ui.Cyan("gez revert  →  revert 중단"))
		} else {
			ui.Fail("revert 실패")
		}
		fmt.Println()
		return
	}

	fmt.Println()
	if noCommit {
		ui.Success("revert 완료 (변경사항이 스테이징됨) — gez c 로 커밋하세요")
	} else {
		ui.Success(fmt.Sprintf("revert 완료! 커밋 '%s' 이 취소됐습니다", hash[:7]))
	}
	fmt.Println()
}
