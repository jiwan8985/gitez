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

var stashCmd = &cobra.Command{
	Use:   "stash",
	Short: "스태시 관리 (push·pop·apply·drop·list)",
	Run: func(cmd *cobra.Command, args []string) {
		if !git.IsRepo() {
			ui.Fail("git 저장소가 아닙니다")
			os.Exit(1)
		}
		runStashMenu()
	},
}

var stashPushCmd = &cobra.Command{
	Use:   "push [메시지]",
	Short: "현재 변경사항을 스태시에 저장",
	Run: func(cmd *cobra.Command, args []string) {
		if !git.IsRepo() {
			ui.Fail("git 저장소가 아닙니다")
			os.Exit(1)
		}
		msg := strings.Join(args, " ")
		doStashPush(msg)
	},
}

var stashPopCmd = &cobra.Command{
	Use:   "pop",
	Short: "가장 최근 스태시를 복원하고 제거",
	Run: func(cmd *cobra.Command, args []string) {
		if !git.IsRepo() {
			ui.Fail("git 저장소가 아닙니다")
			os.Exit(1)
		}
		doStashApplyOrPop("pop")
	},
}

var stashListCmd = &cobra.Command{
	Use:   "list",
	Short: "스태시 목록 보기",
	Run: func(cmd *cobra.Command, args []string) {
		if !git.IsRepo() {
			ui.Fail("git 저장소가 아닙니다")
			os.Exit(1)
		}
		printStashList()
	},
}

func init() {
	stashCmd.AddCommand(stashPushCmd)
	stashCmd.AddCommand(stashPopCmd)
	stashCmd.AddCommand(stashListCmd)
	rootCmd.AddCommand(stashCmd)
}

// ── interactive menu ──────────────────────────────────────────────────────────

func runStashMenu() {
	list := git.StashList()

	fmt.Println()
	if len(list) == 0 {
		fmt.Printf("  %s\n", ui.Dim("스태시가 비어있습니다"))
	} else {
		fmt.Printf("  %s  (%d개)\n", ui.Bold("스태시 목록"), len(list))
		for i, s := range list {
			fmt.Printf("    %s  %s\n", ui.Yellow(fmt.Sprintf("[%d]", i)), ui.Dim(s))
		}
	}
	fmt.Println()

	actions := []string{"스태시에 저장 (push)"}
	if len(list) > 0 {
		actions = append(actions,
			"최근 스태시 복원 + 제거 (pop)",
			"스태시 선택해서 적용 (apply)",
			"스태시 diff 미리보기",
			"스태시 선택해서 삭제 (drop)",
		)
	}
	actions = append(actions, "취소")

	var action string
	if err := survey.AskOne(&survey.Select{
		Message: "작업 선택:",
		Options: actions,
	}, &action); err != nil || action == "취소" {
		return
	}

	switch {
	case strings.HasPrefix(action, "스태시에 저장"):
		var msg string
		_ = survey.AskOne(&survey.Input{Message: "스태시 메시지 (선택사항):"}, &msg)
		doStashPush(strings.TrimSpace(msg))

	case strings.HasPrefix(action, "최근 스태시 복원"):
		doStashApplyOrPop("pop")

	case strings.HasPrefix(action, "스태시 선택해서 적용"):
		if idx := selectStash(list, "적용할 스태시 선택:"); idx >= 0 {
			doStashRef(fmt.Sprintf("stash@{%d}", idx), false)
		}

	case strings.HasPrefix(action, "스태시 diff 미리보기"):
		if idx := selectStash(list, "미리볼 스태시 선택:"); idx >= 0 {
			ref := fmt.Sprintf("stash@{%d}", idx)
			diff := git.StashShow(ref)
			fmt.Println()
			if diff == "" {
				ui.Info("내용 없음")
			} else {
				fmt.Println(diff)
			}
			fmt.Println()
		}

	case strings.HasPrefix(action, "스태시 선택해서 삭제"):
		if idx := selectStash(list, "삭제할 스태시 선택:"); idx >= 0 {
			doStashDrop(fmt.Sprintf("stash@{%d}", idx))
		}
	}
}

// ── helpers ───────────────────────────────────────────────────────────────────

func doStashPush(msg string) {
	lines := git.StatusShort()
	if len(lines) == 0 {
		ui.Info("스태시할 변경사항이 없습니다")
		return
	}

	args := []string{"stash", "push", "--include-untracked"}
	if msg != "" {
		args = append(args, "-m", msg)
	}
	if err := git.RunLive(args...); err != nil {
		ui.Fail("스태시 저장 실패: " + err.Error())
		return
	}
	ui.Success("스태시에 저장했습니다")
}

func doStashApplyOrPop(op string) {
	list := git.StashList()
	if len(list) == 0 {
		ui.Info("스태시가 비어있습니다")
		return
	}
	if err := git.RunLive("stash", op); err != nil {
		ui.Fail(fmt.Sprintf("stash %s 실패", op))
		return
	}
	ui.Success(fmt.Sprintf("stash %s 완료!", op))
}

func doStashRef(ref string, keepIndex bool) {
	args := []string{"stash", "apply"}
	if keepIndex {
		args = append(args, "--index")
	}
	args = append(args, ref)
	if err := git.RunLive(args...); err != nil {
		ui.Fail("스태시 적용 실패")
		return
	}
	ui.Success("스태시 적용 완료!")
}

func doStashDrop(ref string) {
	var ok bool
	_ = survey.AskOne(&survey.Confirm{
		Message: fmt.Sprintf("%s 을(를) 삭제할까요?", ref),
		Default: false,
	}, &ok)
	if !ok {
		return
	}
	if _, err := git.Run("stash", "drop", ref); err != nil {
		ui.Fail("스태시 삭제 실패: " + err.Error())
		return
	}
	ui.Success("스태시 삭제 완료!")
}

func printStashList() {
	list := git.StashList()
	if len(list) == 0 {
		ui.Info("스태시가 비어있습니다")
		return
	}
	fmt.Println()
	fmt.Printf("  %s  (%d개)\n\n", ui.Bold("스태시 목록"), len(list))
	for i, s := range list {
		fmt.Printf("    %s  %s\n", ui.Yellow(fmt.Sprintf("[%d]", i)), s)
	}
	fmt.Println()
}

func selectStash(list []string, prompt string) int {
	if len(list) == 0 {
		return -1
	}
	var selected string
	if err := survey.AskOne(&survey.Select{
		Message: prompt,
		Options: list,
	}, &selected); err != nil {
		return -1
	}
	for i, s := range list {
		if s == selected {
			return i
		}
	}
	return -1
}
