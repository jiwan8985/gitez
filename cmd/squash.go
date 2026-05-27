package cmd

import (
	"fmt"
	"gez/internal/git"
	"gez/internal/ui"
	"os"
	"strconv"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"
)

var squashCmd = &cobra.Command{
	Use:   "squash [n]",
	Short: "최근 N개 커밋을 하나로 합치기",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if !git.IsRepo() {
			ui.Fail("git 저장소가 아닙니다")
			os.Exit(1)
		}
		n := 0
		if len(args) > 0 {
			if v, err := strconv.Atoi(args[0]); err == nil {
				n = v
			}
		}
		runSquash(n)
	},
}

func init() {
	rootCmd.AddCommand(squashCmd)
}

func runSquash(n int) {
	commits := git.RecentCommits(20)
	if len(commits) < 2 {
		ui.Info("squash할 커밋이 충분하지 않습니다 (최소 2개 필요)")
		return
	}

	fmt.Println()
	fmt.Println(ui.Bold("  최근 커밋:"))
	for i, c := range commits {
		fmt.Printf("    %s  %s\n", ui.Dim(fmt.Sprintf("[%d]", i+1)), c)
	}
	fmt.Println()

	if n < 2 {
		// Ask interactively
		options := make([]string, len(commits)-1)
		for i := range options {
			options[i] = fmt.Sprintf("최근 %d개 합치기", i+2)
		}
		var sel string
		if err := survey.AskOne(&survey.Select{
			Message: "몇 개의 커밋을 합칠까요?",
			Options: options,
		}, &sel); err != nil {
			return
		}
		// Parse "최근 N개" -> N
		parts := strings.Fields(sel)
		if len(parts) >= 2 {
			if v, err := strconv.Atoi(parts[1]); err == nil {
				n = v
			}
		}
	}

	if n < 2 {
		ui.Warn("2개 이상을 선택해야 합니다")
		return
	}
	if n > len(commits) {
		n = len(commits)
	}

	// Show what will be squashed
	fmt.Printf("\n  %s\n", ui.Bold(fmt.Sprintf("합쳐질 커밋 (%d개):", n)))
	for i := 0; i < n; i++ {
		fmt.Printf("    %s  %s\n", ui.Yellow("·"), commits[i])
	}
	fmt.Println()

	// New commit message
	oldMsgs := make([]string, 0, n)
	for i := 0; i < n; i++ {
		parts := strings.SplitN(commits[i], " ", 2)
		if len(parts) == 2 {
			oldMsgs = append(oldMsgs, parts[1])
		}
	}
	defaultMsg := oldMsgs[0] // Use newest commit's message as default

	var newMsg string
	if err := survey.AskOne(&survey.Input{
		Message: "합쳐진 커밋의 메시지:",
		Default: defaultMsg,
	}, &newMsg, survey.WithValidator(survey.Required)); err != nil {
		return
	}
	newMsg = strings.TrimSpace(newMsg)

	var ok bool
	if err := survey.AskOne(&survey.Confirm{
		Message: fmt.Sprintf("최근 %d개 커밋을 '%s' 로 합칠까요?", n, newMsg),
		Default: true,
	}, &ok); err != nil || !ok {
		ui.Warn("취소되었습니다")
		return
	}

	fmt.Println()
	// soft reset to N commits ago
	target := fmt.Sprintf("HEAD~%d", n)
	if _, err := git.Run("reset", "--soft", target); err != nil {
		ui.Fail("reset 실패: " + err.Error())
		return
	}
	// commit with new message
	if _, err := git.Run("commit", "-m", newMsg); err != nil {
		ui.Fail("커밋 실패: " + err.Error())
		return
	}

	newHash, _ := git.Run("rev-parse", "--short", "HEAD")
	ui.Success(fmt.Sprintf("✔ %d개 커밋을 [%s] '%s' 로 합쳤습니다", n, newHash, newMsg))
	fmt.Println()

	// Push reminder
	remotes := git.Remotes()
	if len(remotes) > 0 {
		fmt.Printf("  %s  %s  (히스토리 변경으로 force-push 필요)\n\n",
			ui.Dim("⚠ 원격에 push하려면:"),
			ui.Cyan("gez p  (force-with-lease)"))
	}
}
