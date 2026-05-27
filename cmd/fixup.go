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

var fixupCmd = &cobra.Command{
	Use:   "fixup",
	Short: "특정 커밋의 fixup 커밋 생성 (+ 자동 squash 옵션)",
	Run: func(cmd *cobra.Command, args []string) {
		if !git.IsRepo() {
			ui.Fail("git 저장소가 아닙니다")
			os.Exit(1)
		}
		runFixup()
	},
}

func init() {
	rootCmd.AddCommand(fixupCmd)
}

func runFixup() {
	// Check staged files
	staged, _ := git.Run("diff", "--cached", "--name-only")
	staged = strings.TrimSpace(staged)

	fmt.Println()

	if staged == "" {
		// Ask to stage first
		lines := git.StatusShort()
		if len(lines) == 0 {
			ui.Info("변경사항이 없습니다")
			return
		}

		fmt.Println(ui.Bold("  현재 변경사항:"))
		var unstaged []string
		for _, l := range lines {
			if len(l) < 3 {
				continue
			}
			fmt.Printf("    %s  %s\n", ui.ColorXY(l[:2]), l[3:])
			if l[1] != ' ' {
				unstaged = append(unstaged, l[3:])
			}
		}
		fmt.Println()

		var selected []string
		if err := survey.AskOne(&survey.MultiSelect{
			Message: "fixup에 포함할 파일 선택:",
			Options: unstaged,
		}, &selected); err != nil || len(selected) == 0 {
			ui.Warn("취소되었습니다")
			return
		}
		for _, f := range selected {
			if _, err := git.Run("add", f); err != nil {
				ui.Fail("stage 실패: " + err.Error())
				return
			}
		}
		staged, _ = git.Run("diff", "--cached", "--name-only")
	}

	// Show staged files
	fmt.Println(ui.Bold("  fixup에 포함될 파일:"))
	for _, f := range strings.Split(staged, "\n") {
		if f != "" {
			fmt.Printf("    %s  %s\n", ui.Green("✔"), f)
		}
	}
	fmt.Println()

	// Pick target commit
	commits := git.RecentCommits(20)
	if len(commits) == 0 {
		ui.Info("커밋 기록이 없습니다")
		return
	}

	var sel string
	if err := survey.AskOne(&survey.Select{
		Message: "어느 커밋을 수정하는 fixup인가요?",
		Options: commits,
		Help:    "선택한 커밋에 현재 스테이징된 변경사항이 포함됩니다",
	}, &sel); err != nil {
		return
	}

	targetHash := strings.Fields(sel)[0]
	targetMsg := ""
	if len(strings.Fields(sel)) > 1 {
		targetMsg = strings.Join(strings.Fields(sel)[1:], " ")
	}

	// Create fixup commit
	fixupMsg := "fixup! " + targetMsg
	if _, err := git.Run("commit", "-m", fixupMsg); err != nil {
		ui.Fail("fixup 커밋 생성 실패: " + err.Error())
		return
	}

	newHash, _ := git.Run("rev-parse", "--short", "HEAD")
	ui.Success(fmt.Sprintf("fixup 커밋 생성: [%s] %s", newHash, fixupMsg))
	fmt.Println()

	// Offer autosquash
	var doSquash bool
	_ = survey.AskOne(&survey.Confirm{
		Message: fmt.Sprintf("지금 바로 '%s' 에 squash할까요? (rebase --autosquash)", targetHash[:7]),
		Default: false,
	}, &doSquash)

	if doSquash {
		fmt.Println()
		ui.Info("rebase --autosquash 실행 중…")
		fmt.Printf("  %s  GIT_SEQUENCE_EDITOR=true 로 자동 처리합니다\n\n", ui.Dim("Tip:"))

		// Count commits to go back: find targetHash in log
		out, _ := git.Run("log", "--oneline", fmt.Sprintf("%s..HEAD", targetHash))
		n := len(strings.Split(strings.TrimSpace(out), "\n")) + 1

		// Set GIT_SEQUENCE_EDITOR=true to skip interactive editor
		os.Setenv("GIT_SEQUENCE_EDITOR", "true")
		err := git.RunLive("rebase", "-i", "--autosquash", fmt.Sprintf("HEAD~%d", n))
		os.Unsetenv("GIT_SEQUENCE_EDITOR")

		if err != nil {
			ui.Warn("rebase 충돌 발생 — 충돌 해결 후 git rebase --continue")
		} else {
			ui.Success("autosquash 완료!")
		}
	} else {
		fmt.Printf("  %s  나중에 실행: %s\n\n",
			ui.Dim("Tip:"),
			ui.Cyan(fmt.Sprintf("git rebase -i --autosquash %s~1", targetHash)))
	}
}
