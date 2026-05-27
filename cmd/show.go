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

var showCmd = &cobra.Command{
	Use:   "show [hash]",
	Short: "커밋 상세 보기 (변경파일·diff·메타데이터)",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if !git.IsRepo() {
			ui.Fail("git 저장소가 아닙니다")
			os.Exit(1)
		}
		hash := ""
		if len(args) > 0 {
			hash = args[0]
		}
		runShow(hash)
	},
}

func init() {
	rootCmd.AddCommand(showCmd)
}

func runShow(hash string) {
	if hash == "" {
		// Interactive: pick from recent commits
		commits := git.RecentCommits(30)
		if len(commits) == 0 {
			ui.Info("커밋 기록이 없습니다")
			return
		}
		if err := survey.AskOne(&survey.Select{
			Message: "보여줄 커밋 선택:",
			Options: commits,
		}, &hash); err != nil {
			return
		}
		// Extract hash from "abc1234 subject"
		hash = strings.Fields(hash)[0]
	}

	fmt.Println()

	// Show metadata header
	meta, err := git.Run("show",
		"--no-patch",
		"--format=커밋: %C(yellow)%H%C(reset)%n작성자: %an <%ae>%n날짜:   %ad%n%n    %s%n%n%b",
		"--date=format:%Y-%m-%d %H:%M:%S",
		hash)
	if err != nil {
		ui.Fail("커밋을 찾을 수 없습니다: " + hash)
		return
	}
	fmt.Println(meta)

	// Show stat summary
	stat, _ := git.Run("show", "--stat", "--no-patch", hash)
	if stat != "" {
		fmt.Printf("  %s\n", ui.Bold("변경 요약:"))
		for _, l := range strings.Split(stat, "\n") {
			if l != "" {
				fmt.Printf("  %s\n", l)
			}
		}
	}
	fmt.Println()

	// Ask for full diff
	var showDiff bool
	_ = survey.AskOne(&survey.Confirm{
		Message: "전체 diff 보기?",
		Default: false,
	}, &showDiff)

	if showDiff {
		fmt.Println()
		_ = git.RunLive("show", "--color=always", hash)
		fmt.Println()
	}
}
