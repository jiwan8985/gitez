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

var cherryPickCmd = &cobra.Command{
	Use:     "cherry-pick [해시...]",
	Aliases: []string{"cp"},
	Short:   "다른 브랜치의 커밋을 현재 브랜치에 적용",
	Run: func(cmd *cobra.Command, args []string) {
		if !git.IsRepo() {
			ui.Fail("git 저장소가 아닙니다")
			os.Exit(1)
		}
		if git.IsCherryPickInProgress() {
			runCherryPickInProgress()
			return
		}
		if len(args) > 0 {
			doCherryPick(args)
		} else {
			runCherryPickInteractive()
		}
	},
}

func init() {
	rootCmd.AddCommand(cherryPickCmd)
}

// ── 진행 중 처리 ──────────────────────────────────────────────────────────────

func runCherryPickInProgress() {
	ui.Warn("cherry-pick이 진행 중입니다")
	fmt.Println()

	var action string
	if err := survey.AskOne(&survey.Select{
		Message: "작업 선택:",
		Options: []string{
			"계속 진행  (cherry-pick --continue)",
			"이 커밋 건너뛰기  (cherry-pick --skip)",
			"cherry-pick 중단  (cherry-pick --abort)",
		},
	}, &action); err != nil {
		return
	}

	fmt.Println()
	switch {
	case strings.HasPrefix(action, "계속"):
		if err := git.RunLive("cherry-pick", "--continue"); err != nil {
			ui.Warn("계속 실패 — 충돌 파일을 모두 해결하고 스테이징했는지 확인하세요")
		} else {
			ui.Success("cherry-pick 계속 완료!")
		}
	case strings.HasPrefix(action, "이 커밋"):
		if err := git.RunLive("cherry-pick", "--skip"); err != nil {
			ui.Fail("--skip 실패")
		} else {
			ui.Success("커밋을 건너뛰었습니다")
		}
	case strings.HasPrefix(action, "cherry-pick 중단"):
		if err := git.RunLive("cherry-pick", "--abort"); err != nil {
			ui.Fail("--abort 실패")
		} else {
			ui.Success("cherry-pick 중단 — 원래 상태로 복구됐습니다")
		}
	}
	fmt.Println()
}

// ── 대화형: 브랜치 → 커밋 선택 ───────────────────────────────────────────────

func runCherryPickInteractive() {
	current := git.CurrentBranch()
	locals := git.LocalBranches()

	fmt.Println()
	fmt.Printf("  %s  %s\n\n", ui.Bold("현재 브랜치:"), ui.BoldCyan(current))

	// 소스 브랜치 선택
	var options []string
	for _, b := range locals {
		if b != current {
			options = append(options, b)
		}
	}
	for _, r := range git.RemoteBranches() {
		options = append(options, fmt.Sprintf("%s  %s", r, ui.Dim("(remote)")))
	}
	if len(options) == 0 {
		ui.Info("커밋을 가져올 다른 브랜치가 없습니다")
		return
	}

	var sourceSel string
	if err := survey.AskOne(&survey.Select{
		Message: "커밋을 가져올 브랜치:",
		Options: options,
	}, &sourceSel); err != nil {
		return
	}
	source := strings.Fields(sourceSel)[0]

	// 해당 브랜치에만 있는 커밋 목록
	commits := git.CommitsBetween(current, source)
	if len(commits) == 0 {
		ui.Info(fmt.Sprintf("'%s' 에 현재 브랜치에 없는 커밋이 없습니다", source))
		return
	}

	fmt.Println()
	var selected []string
	if err := survey.AskOne(&survey.MultiSelect{
		Message: "cherry-pick 할 커밋 선택 (Space=선택, Enter=확인):",
		Options: commits,
	}, &selected); err != nil || len(selected) == 0 {
		ui.Warn("선택된 커밋이 없습니다")
		return
	}

	// 오래된 커밋부터 적용 (선택 역순)
	hashes := make([]string, 0, len(selected))
	for i := len(selected) - 1; i >= 0; i-- {
		hashes = append(hashes, strings.Fields(selected[i])[0])
	}

	doCherryPick(hashes)
}

// ── 실행 ─────────────────────────────────────────────────────────────────────

func doCherryPick(hashes []string) {
	fmt.Println()
	fmt.Printf("  %s  %d개 커밋 cherry-pick 중...\n\n",
		ui.Bold("Cherry-pick:"), len(hashes))

	args := append([]string{"cherry-pick"}, hashes...)
	if err := git.RunLive(args...); err != nil {
		fmt.Println()
		if git.IsCherryPickInProgress() {
			ui.Warn("충돌 발생 — 충돌 파일 수정 후:")
			fmt.Printf("    %s  스테이징 후 계속\n", ui.Cyan("gez cp  →  계속 진행"))
			fmt.Printf("    %s  취소\n", ui.Cyan("gez cp  →  cherry-pick 중단"))
		} else {
			ui.Fail("cherry-pick 실패")
		}
		fmt.Println()
		return
	}

	fmt.Println()
	ui.Success(fmt.Sprintf("%d개 커밋 cherry-pick 완료!", len(hashes)))
	fmt.Println()
}
