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

var logCount int
var logInteractive bool

var logCmd = &cobra.Command{
	Use:     "log",
	Aliases: []string{"l"},
	Short:   "커밋 그래프 로그 보기 [-i: 대화형 선택]",
	Run: func(cmd *cobra.Command, args []string) {
		if !git.IsRepo() {
			ui.Fail("git 저장소가 아닙니다")
			os.Exit(1)
		}
		if logInteractive {
			runLogInteractive()
		} else {
			runLog()
		}
	},
}

func init() {
	logCmd.Flags().IntVarP(&logCount, "count", "n", 20, "표시할 커밋 수")
	logCmd.Flags().BoolVarP(&logInteractive, "interactive", "i", false, "커밋 선택 후 작업 수행")
	rootCmd.AddCommand(logCmd)
}

// runLog shows the graph log directly via git.
func runLog() {
	format := "%C(yellow)%h%C(reset)  %C(cyan)%ad%C(reset)  %C(bold white)%s%C(reset)  %C(dim)%an%C(reset)"
	fmt.Println()
	err := git.RunLive(
		"log",
		"--graph",
		"--color=always",
		fmt.Sprintf("--pretty=format:%s", format),
		"--date=short",
		fmt.Sprintf("-n%d", logCount),
		"--decorate",
	)
	if err != nil {
		ui.Fail("로그 조회 실패")
		return
	}
	fmt.Println()
	fmt.Println()
}

// runLogInteractive lets the user pick a commit and perform an action.
func runLogInteractive() {
	commits := git.RecentCommits(logCount)
	if len(commits) == 0 {
		ui.Info("커밋 기록이 없습니다")
		return
	}

	fmt.Println()
	var selected string
	if err := survey.AskOne(&survey.Select{
		Message: "커밋 선택:",
		Options: commits,
		Help:    "hash  커밋 메시지 형식",
	}, &selected); err != nil {
		return
	}

	hash := strings.Fields(selected)[0]

	fmt.Println()
	var action string
	if err := survey.AskOne(&survey.Select{
		Message: "작업 선택:",
		Options: []string{
			"상세 보기 (show)",
			"파일 목록 (diff-tree)",
			"이 커밋으로 cherry-pick",
			"이 커밋으로 soft reset",
			"이 커밋으로 mixed reset",
			"취소",
		},
	}, &action); err != nil || action == "취소" {
		return
	}

	fmt.Println()
	switch {
	case strings.HasPrefix(action, "상세 보기"):
		_ = git.RunLive("show", "--stat", "--color=always", hash)

	case strings.HasPrefix(action, "파일 목록"):
		_ = git.RunLive("diff-tree", "--no-commit-id", "-r", "--name-status", "--color=always", hash)

	case strings.HasPrefix(action, "cherry-pick"):
		if err := git.RunLive("cherry-pick", hash); err != nil {
			ui.Warn("cherry-pick 충돌 — 충돌 해결 후 git cherry-pick --continue")
		} else {
			ui.Success(fmt.Sprintf("cherry-pick 완료: %s", hash))
		}

	case strings.HasPrefix(action, "soft reset"):
		var ok bool
		_ = survey.AskOne(&survey.Confirm{
			Message: fmt.Sprintf("'%s' 이전으로 soft reset 할까요? (커밋 취소, 변경사항 유지)", hash),
			Default: false,
		}, &ok)
		if ok {
			if _, err := git.Run("reset", "--soft", hash+"^"); err != nil {
				// try without ^ (reset to the commit itself)
				_, err = git.Run("reset", "--soft", hash)
				if err != nil {
					ui.Fail("reset 실패: " + err.Error())
					return
				}
			}
			ui.Success("soft reset 완료")
		}

	case strings.HasPrefix(action, "mixed reset"):
		var ok bool
		_ = survey.AskOne(&survey.Confirm{
			Message: fmt.Sprintf("'%s' 이전으로 mixed reset 할까요? (커밋+스테이징 취소)", hash),
			Default: false,
		}, &ok)
		if ok {
			if _, err := git.Run("reset", hash+"^"); err != nil {
				_, err = git.Run("reset", hash)
				if err != nil {
					ui.Fail("reset 실패: " + err.Error())
					return
				}
			}
			ui.Success("mixed reset 완료")
		}
	}
	fmt.Println()
}
