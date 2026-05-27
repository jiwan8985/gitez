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

var diffCmd = &cobra.Command{
	Use:     "diff [파일...]",
	Aliases: []string{"d"},
	Short:   "변경사항 diff 보기",
	Run: func(cmd *cobra.Command, args []string) {
		if !git.IsRepo() {
			ui.Fail("git 저장소가 아닙니다")
			os.Exit(1)
		}
		runDiff(args)
	},
}

func init() {
	rootCmd.AddCommand(diffCmd)
}

func runDiff(files []string) {
	// If file args given, just show diff for those files
	if len(files) > 0 {
		args := append([]string{"diff", "--color=always", "HEAD", "--"}, files...)
		fmt.Println()
		_ = git.RunLive(args...)
		fmt.Println()
		return
	}

	// Check what's available
	lines := git.StatusShort()
	if len(lines) == 0 {
		ui.Info("변경사항이 없습니다")
		return
	}

	hasStagedChanges := false
	hasUnstagedChanges := false
	for _, l := range lines {
		if len(l) < 2 {
			continue
		}
		x, y := l[0], l[1]
		if x == '?' && y == '?' {
			continue // untracked → no diff
		}
		if x != ' ' && x != '?' {
			hasStagedChanges = true
		}
		if y != ' ' && y != '?' {
			hasUnstagedChanges = true
		}
	}

	var options []string
	if hasUnstagedChanges {
		options = append(options, "워킹 트리 변경사항  (git diff)")
	}
	if hasStagedChanges {
		options = append(options, "스테이징된 변경사항  (git diff --cached)")
	}
	if hasStagedChanges || hasUnstagedChanges {
		options = append(options, "모든 변경사항  (git diff HEAD)")
	}
	options = append(options, "특정 파일 선택")
	options = append(options, "취소")

	var choice string
	if err := survey.AskOne(&survey.Select{
		Message: "diff 종류 선택:",
		Options: options,
	}, &choice); err != nil || choice == "취소" {
		return
	}

	fmt.Println()
	switch {
	case strings.HasPrefix(choice, "워킹 트리"):
		_ = git.RunLive("diff", "--color=always")

	case strings.HasPrefix(choice, "스테이징된"):
		_ = git.RunLive("diff", "--cached", "--color=always")

	case strings.HasPrefix(choice, "모든 변경"):
		_ = git.RunLive("diff", "HEAD", "--color=always")

	case strings.HasPrefix(choice, "특정 파일"):
		runDiffFileSelect(lines)
	}
	fmt.Println()
}

func runDiffFileSelect(lines []string) {
	var fileOptions []string
	for _, l := range lines {
		if len(l) < 3 {
			continue
		}
		x, y := l[0], l[1]
		if x == '?' && y == '?' {
			continue
		}
		fileOptions = append(fileOptions, l[3:])
	}

	if len(fileOptions) == 0 {
		ui.Info("diff 가능한 파일이 없습니다")
		return
	}

	var selected []string
	if err := survey.AskOne(&survey.MultiSelect{
		Message: "파일 선택 (Space=선택, Enter=확인):",
		Options: fileOptions,
	}, &selected); err != nil || len(selected) == 0 {
		return
	}

	args := append([]string{"diff", "--color=always", "HEAD", "--"}, selected...)
	_ = git.RunLive(args...)
}
