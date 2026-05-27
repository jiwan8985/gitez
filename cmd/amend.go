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

var amendCmd = &cobra.Command{
	Use:   "amend",
	Short: "마지막 커밋 수정 (메시지·파일 추가)",
	Run: func(cmd *cobra.Command, args []string) {
		if !git.IsRepo() {
			ui.Fail("git 저장소가 아닙니다")
			os.Exit(1)
		}
		runAmend()
	},
}

func init() {
	rootCmd.AddCommand(amendCmd)
}

func runAmend() {
	// Show last commit
	lastCommit := git.RecentCommits(1)
	lastHash, _ := git.Run("rev-parse", "--short", "HEAD")
	lastMsg, _ := git.Run("log", "-1", "--format=%s")
	lastBody, _ := git.Run("log", "-1", "--format=%b")

	fmt.Println()
	fmt.Printf("  %s  %s\n", ui.Bold("마지막 커밋:"), ui.Cyan("["+lastHash+"]"))
	if len(lastCommit) > 0 {
		fmt.Printf("  %s\n", lastCommit[0])
	}
	fmt.Println()

	// Show files in last commit
	files, _ := git.Run("diff-tree", "--no-commit-id", "-r", "--name-only", "HEAD")
	if files != "" {
		fmt.Printf("  %s\n", ui.Bold("포함된 파일:"))
		for _, f := range strings.Split(files, "\n") {
			if f != "" {
				fmt.Printf("    %s  %s\n", ui.Dim("·"), f)
			}
		}
		fmt.Println()
	}

	// Choose what to amend
	var action string
	if err := survey.AskOne(&survey.Select{
		Message: "수정 항목:",
		Options: []string{
			"메시지만 수정",
			"현재 스테이징된 파일 추가 후 메시지 수정",
			"현재 스테이징된 파일 추가 (메시지 유지)",
			"취소",
		},
	}, &action); err != nil || action == "취소" {
		return
	}

	fmt.Println()

	switch {
	case strings.HasPrefix(action, "메시지만 수정"):
		newMsg := amendMessage(lastMsg, lastBody)
		if newMsg == "" {
			return
		}
		if _, err := git.Run("commit", "--amend", "-m", newMsg); err != nil {
			ui.Fail("amend 실패: " + err.Error())
			return
		}
		ui.Success("커밋 메시지 수정 완료!")

	case strings.HasPrefix(action, "현재 스테이징된 파일 추가 후 메시지"):
		if !checkStagedFiles() {
			return
		}
		newMsg := amendMessage(lastMsg, lastBody)
		if newMsg == "" {
			return
		}
		if _, err := git.Run("commit", "--amend", "-m", newMsg); err != nil {
			ui.Fail("amend 실패: " + err.Error())
			return
		}
		ui.Success("파일 추가 + 메시지 수정 완료!")

	case strings.HasPrefix(action, "현재 스테이징된 파일 추가 (메시지 유지)"):
		if !checkStagedFiles() {
			return
		}
		if _, err := git.Run("commit", "--amend", "--no-edit"); err != nil {
			ui.Fail("amend 실패: " + err.Error())
			return
		}
		ui.Success("파일 추가 완료! (메시지 유지)")
	}

	newHash, _ := git.Run("rev-parse", "--short", "HEAD")
	fmt.Printf("  %s  [%s]\n", ui.Dim("새 커밋:"), ui.Yellow(newHash))

	remotes := git.Remotes()
	if len(remotes) > 0 {
		fmt.Printf("\n  %s  %s\n", ui.Dim("⚠ 이미 push된 경우:"),
			ui.Cyan("gez p  (force-with-lease로 push)"))
	}
	fmt.Println()
}

func amendMessage(lastMsg, lastBody string) string {
	current := lastMsg
	if strings.TrimSpace(lastBody) != "" {
		current = lastMsg + "\n\n" + strings.TrimSpace(lastBody)
	}

	var newMsg string
	if err := survey.AskOne(&survey.Input{
		Message: "새 커밋 메시지:",
		Default: current,
	}, &newMsg, survey.WithValidator(survey.Required)); err != nil {
		return ""
	}
	return strings.TrimSpace(newMsg)
}

func checkStagedFiles() bool {
	staged, _ := git.Run("diff", "--cached", "--name-only")
	if strings.TrimSpace(staged) == "" {
		ui.Warn("스테이징된 파일이 없습니다. git add 로 파일을 먼저 스테이징하세요")
		return false
	}
	fmt.Println(ui.Bold("  추가될 파일:"))
	for _, f := range strings.Split(staged, "\n") {
		if f != "" {
			fmt.Printf("    %s  %s\n", ui.Green("✔"), f)
		}
	}
	fmt.Println()
	return true
}
