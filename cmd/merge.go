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

var mergeCmd = &cobra.Command{
	Use:   "merge [브랜치]",
	Short: "브랜치 병합",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if !git.IsRepo() {
			ui.Fail("git 저장소가 아닙니다")
			os.Exit(1)
		}
		if len(args) > 0 {
			doMerge(args[0])
		} else {
			runMergeInteractive()
		}
	},
}

func init() {
	rootCmd.AddCommand(mergeCmd)
}

func runMergeInteractive() {
	current := git.CurrentBranch()
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
			options = append(options, fmt.Sprintf("%s  %s", short, ui.Dim("(remote)")))
		}
	}

	if len(options) == 0 {
		ui.Info("병합할 수 있는 다른 브랜치가 없습니다")
		return
	}

	fmt.Println()
	fmt.Printf("  %s  %s\n\n", ui.Bold("현재 브랜치:"), ui.BoldCyan(current))

	var selected string
	if err := survey.AskOne(&survey.Select{
		Message: "병합할 브랜치 선택:",
		Options: options,
	}, &selected); err != nil {
		return
	}

	branch := strings.Fields(selected)[0]
	doMerge(branch)
}

func doMerge(branch string) {
	current := git.CurrentBranch()

	// Preview: commits that will be merged
	preview, _ := git.Run("log", "--oneline", fmt.Sprintf("%s..%s", current, branch))
	fmt.Println()
	if preview != "" {
		fmt.Printf("  %s  병합될 커밋 (%s → %s):\n", ui.Bold("Preview:"), ui.Cyan(branch), ui.BoldCyan(current))
		for _, l := range strings.Split(preview, "\n") {
			if l != "" {
				fmt.Printf("    %s  %s\n", ui.Yellow("·"), l)
			}
		}
		fmt.Println()
	} else {
		fmt.Printf("  %s  병합할 새 커밋이 없습니다 (%s 이미 최신)\n\n", ui.Dim("Info:"), current)
	}

	var ok bool
	if err := survey.AskOne(&survey.Confirm{
		Message: fmt.Sprintf("'%s' 를 '%s' 에 병합할까요?", branch, current),
		Default: true,
	}, &ok); err != nil || !ok {
		ui.Warn("취소되었습니다")
		return
	}

	fmt.Println()
	if err := git.RunLive("merge", branch); err != nil {
		fmt.Println()
		ui.Warn("병합 중 충돌이 발생했습니다")
		fmt.Printf("  %s  충돌 파일을 수정한 후:\n", ui.Dim("해결 방법:"))
		fmt.Printf("    %s  → 충돌 해결 후 스테이징\n", ui.Cyan("gez c"))
		fmt.Printf("    %s  → 병합 중단\n", ui.Cyan("git merge --abort"))
		fmt.Println()
		return
	}
	fmt.Println()
	ui.Success(fmt.Sprintf("'%s' 병합 완료!", branch))
	fmt.Println()
}
