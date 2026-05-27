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

var bisectCmd = &cobra.Command{
	Use:   "bisect",
	Short: "이진 탐색으로 버그 도입 커밋 찾기",
	Run: func(cmd *cobra.Command, args []string) {
		if !git.IsRepo() {
			ui.Fail("git 저장소가 아닙니다")
			os.Exit(1)
		}
		runBisectMenu()
	},
}

var bisectStartCmd = &cobra.Command{
	Use:   "start [bad] [good]",
	Short: "bisect 시작",
	Args:  cobra.MaximumNArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		bad := ""
		good := ""
		if len(args) > 0 {
			bad = args[0]
		}
		if len(args) > 1 {
			good = args[1]
		}
		doBisectStart(bad, good)
	},
}

var bisectGoodCmd = &cobra.Command{
	Use:   "good [rev]",
	Short: "현재(또는 지정) 커밋을 good으로 표시",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		gArgs := []string{"bisect", "good"}
		gArgs = append(gArgs, args...)
		fmt.Println()
		if err := git.RunLive(gArgs...); err != nil {
			ui.Fail("bisect good 실패")
		}
		fmt.Println()
	},
}

var bisectBadCmd = &cobra.Command{
	Use:   "bad [rev]",
	Short: "현재(또는 지정) 커밋을 bad로 표시",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		bArgs := []string{"bisect", "bad"}
		bArgs = append(bArgs, args...)
		fmt.Println()
		if err := git.RunLive(bArgs...); err != nil {
			ui.Fail("bisect bad 실패")
		}
		fmt.Println()
	},
}

var bisectSkipCmd = &cobra.Command{
	Use:   "skip [rev]",
	Short: "현재 커밋 건너뛰기 (테스트 불가)",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		sArgs := []string{"bisect", "skip"}
		sArgs = append(sArgs, args...)
		fmt.Println()
		_ = git.RunLive(sArgs...)
		fmt.Println()
	},
}

var bisectResetCmd = &cobra.Command{
	Use:   "reset",
	Short: "bisect 종료 및 원래 브랜치로 복귀",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println()
		if err := git.RunLive("bisect", "reset"); err != nil {
			ui.Fail("bisect reset 실패")
		} else {
			ui.Success("bisect 종료, 원래 브랜치로 복귀했습니다")
		}
		fmt.Println()
	},
}

var bisectRunCmd = &cobra.Command{
	Use:   "run <테스트명령>",
	Short: "자동 bisect 실행 (명령이 0 반환=good, 1-127=bad)",
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println()
		ui.Info(fmt.Sprintf("자동 bisect: %s", strings.Join(args, " ")))
		fmt.Println()
		runArgs := append([]string{"bisect", "run"}, args...)
		if err := git.RunLive(runArgs...); err != nil {
			ui.Fail("bisect run 실패")
		}
		fmt.Println()
	},
}

var bisectLogViewCmd = &cobra.Command{
	Use:   "log",
	Short: "bisect 진행 로그 보기",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println()
		_ = git.RunLive("bisect", "log")
		fmt.Println()
	},
}

func init() {
	bisectCmd.AddCommand(bisectStartCmd)
	bisectCmd.AddCommand(bisectGoodCmd)
	bisectCmd.AddCommand(bisectBadCmd)
	bisectCmd.AddCommand(bisectSkipCmd)
	bisectCmd.AddCommand(bisectResetCmd)
	bisectCmd.AddCommand(bisectRunCmd)
	bisectCmd.AddCommand(bisectLogViewCmd)
	rootCmd.AddCommand(bisectCmd)
}

// ── 대화형 메뉴 ───────────────────────────────────────────────────────────────

func runBisectMenu() {
	inProgress := git.IsBisectInProgress()

	fmt.Println()
	if inProgress {
		fmt.Printf("  %s  bisect 진행 중\n\n", ui.BoldYellow("⚡"))
		_ = git.RunLive("bisect", "log")
		fmt.Println()

		var action string
		if err := survey.AskOne(&survey.Select{
			Message: "다음 작업:",
			Options: []string{
				"현재 커밋이 good (버그 없음)",
				"현재 커밋이 bad (버그 있음)",
				"현재 커밋 건너뛰기 (skip)",
				"자동 bisect run",
				"bisect 종료 (reset)",
			},
		}, &action); err != nil {
			return
		}

		fmt.Println()
		switch {
		case strings.HasPrefix(action, "현재 커밋이 good"):
			if err := git.RunLive("bisect", "good"); err != nil {
				ui.Fail("bisect good 실패")
			}
		case strings.HasPrefix(action, "현재 커밋이 bad"):
			if err := git.RunLive("bisect", "bad"); err != nil {
				ui.Fail("bisect bad 실패")
			}
		case strings.HasPrefix(action, "현재 커밋 건너뛰기"):
			_ = git.RunLive("bisect", "skip")
		case strings.HasPrefix(action, "자동 bisect run"):
			var testCmd string
			if err := survey.AskOne(&survey.Input{
				Message: "테스트 명령어:",
				Help:    "exit 0 = good, exit 1-127 = bad. 예: go test ./...",
			}, &testCmd, survey.WithValidator(survey.Required)); err != nil {
				return
			}
			_ = git.RunLive("bisect", "run", "sh", "-c", testCmd)
		case strings.HasPrefix(action, "bisect 종료"):
			if err := git.RunLive("bisect", "reset"); err == nil {
				ui.Success("bisect 종료")
			}
		}
		fmt.Println()
		return
	}

	// Not in progress — guide through start
	fmt.Printf("  %s\n", ui.Bold("Git Bisect — 이진 탐색으로 버그 도입 커밋 찾기"))
	fmt.Println()
	fmt.Printf("  %s\n", ui.Dim("버그가 없는 커밋(good)과 있는 커밋(bad) 사이를 이진 탐색합니다"))
	fmt.Printf("  %s\n", ui.Dim("각 단계에서 현재 커밋을 테스트하고 good/bad 를 표시하세요"))
	fmt.Println()

	commits := git.RecentCommits(30)

	// Pick bad commit (default = HEAD)
	badOptions := append([]string{"현재 커밋 (HEAD)"}, commits...)
	var badSel string
	if err := survey.AskOne(&survey.Select{
		Message: "버그가 있는(bad) 커밋:",
		Options: badOptions,
	}, &badSel); err != nil {
		return
	}
	bad := ""
	if badSel != "현재 커밋 (HEAD)" {
		bad = strings.Fields(badSel)[0]
	}

	// Pick good commit
	var goodSel string
	if err := survey.AskOne(&survey.Select{
		Message: "버그가 없던(good) 커밋:",
		Options: commits,
		Help:    "bad보다 오래된 커밋이어야 합니다",
	}, &goodSel); err != nil {
		return
	}
	good := strings.Fields(goodSel)[0]

	doBisectStart(bad, good)
}

func doBisectStart(bad, good string) {
	fmt.Println()
	args := []string{"bisect", "start"}
	if bad != "" {
		args = append(args, bad)
	}
	if bad != "" && good != "" {
		args = append(args, good)
	}
	if err := git.RunLive(args...); err != nil {
		ui.Fail("bisect start 실패")
		return
	}
	if good != "" && bad == "" {
		// Start then mark good separately
		if err := git.RunLive("bisect", "good", good); err != nil {
			ui.Fail("bisect good 실패")
			return
		}
	}
	fmt.Println()
	ui.Success("bisect 시작! 현재 체크아웃된 커밋을 테스트하세요")
	fmt.Printf("  %s  %s  (good)\n", ui.Dim("테스트 후:"), ui.Cyan("gez bisect good / bad"))
	fmt.Printf("  %s  %s  (자동화)\n", ui.Dim("자동 실행:"), ui.Cyan("gez bisect run <명령>"))
	fmt.Printf("  %s  %s  (종료)\n\n", ui.Dim("완료 후:"), ui.Cyan("gez bisect reset"))
}
