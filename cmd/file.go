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

var fileCmd = &cobra.Command{
	Use:   "file [경로]",
	Short: "파일별 히스토리·blame·복원 통합 메뉴",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if !git.IsRepo() {
			ui.Fail("git 저장소가 아닙니다")
			os.Exit(1)
		}
		path := ""
		if len(args) > 0 {
			path = args[0]
		}
		runFileMenu(path)
	},
}

func init() {
	rootCmd.AddCommand(fileCmd)
}

func runFileMenu(path string) {
	if path == "" {
		// Let user pick a tracked file
		tracked := git.TrackedFiles()
		changed := git.UnstagedFiles()

		// Put changed files first
		seen := make(map[string]bool)
		var options []string
		for _, f := range changed {
			options = append(options, ui.Yellow("* ")+f)
			seen[f] = true
		}
		for _, f := range tracked {
			if !seen[f] {
				options = append(options, "  "+f)
			}
		}
		if len(options) == 0 {
			ui.Info("tracked 파일이 없습니다")
			return
		}

		var sel string
		if err := survey.AskOne(&survey.Select{
			Message: "파일 선택:",
			Options: options,
			Help:    "* 표시 = 변경된 파일",
		}, &sel); err != nil {
			return
		}
		path = strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(sel, "* "), "  "))
	}

	fmt.Println()
	fmt.Printf("  %s  %s\n\n", ui.Bold("파일:"), ui.BoldCyan(path))

	var action string
	if err := survey.AskOne(&survey.Select{
		Message: "작업 선택:",
		Options: []string{
			"히스토리 보기 (커밋 목록)",
			"변경 내용 보기 (diff)",
			"blame — 줄별 작성자 보기",
			"특정 커밋 시점의 내용 보기",
			"파일 복원 (HEAD로)",
			"파일 복원 (특정 커밋으로)",
			"취소",
		},
	}, &action); err != nil || action == "취소" {
		return
	}

	fmt.Println()
	switch {
	case strings.HasPrefix(action, "히스토리"):
		runFileHistory(path)

	case strings.HasPrefix(action, "변경 내용"):
		_ = git.RunLive("diff", "--color=always", "--", path)
		fmt.Println()

	case strings.HasPrefix(action, "blame"):
		_ = git.RunLive("blame", "--color-lines", "--date=short", "-w", "--", path)
		fmt.Println()

	case strings.HasPrefix(action, "특정 커밋 시점"):
		runFileShowAtCommit(path)

	case strings.HasPrefix(action, "파일 복원 (HEAD"):
		var ok bool
		_ = survey.AskOne(&survey.Confirm{
			Message: fmt.Sprintf("'%s' 를 HEAD 상태로 복원할까요? (변경사항 삭제)", path),
			Default: false,
		}, &ok)
		if ok {
			if err := git.RunLive("restore", "--", path); err != nil {
				ui.Fail("복원 실패")
			} else {
				ui.Success("복원 완료")
			}
		}

	case strings.HasPrefix(action, "파일 복원 (특정"):
		runFileRestoreFromCommit(path)
	}
}

func runFileHistory(path string) {
	commits, _ := git.Run("log", "--oneline", "--follow", "-20", "--", path)
	if commits == "" {
		ui.Info("히스토리가 없습니다")
		return
	}

	lines := strings.Split(strings.TrimSpace(commits), "\n")
	fmt.Printf("  %s  (최근 %d개)\n\n", ui.Bold("커밋 히스토리:"), len(lines))
	for _, l := range lines {
		parts := strings.SplitN(l, " ", 2)
		if len(parts) == 2 {
			fmt.Printf("  %s  %s\n", ui.Yellow(parts[0]), parts[1])
		}
	}
	fmt.Println()

	var sel string
	if err := survey.AskOne(&survey.Select{
		Message: "커밋 선택해서 상세 보기:",
		Options: append(lines, "취소"),
	}, &sel); err != nil || sel == "취소" {
		return
	}
	hash := strings.Fields(sel)[0]
	fmt.Println()
	_ = git.RunLive("show", "--color=always", "--stat", hash, "--", path)
	fmt.Println()
}

func runFileShowAtCommit(path string) {
	commits, _ := git.Run("log", "--oneline", "--follow", "-20", "--", path)
	if commits == "" {
		ui.Info("히스토리가 없습니다")
		return
	}
	lines := strings.Split(strings.TrimSpace(commits), "\n")
	var sel string
	if err := survey.AskOne(&survey.Select{
		Message: "시점 선택:",
		Options: lines,
	}, &sel); err != nil {
		return
	}
	hash := strings.Fields(sel)[0]
	fmt.Println()
	_ = git.RunLive("show", hash+":"+path)
	fmt.Println()
}

func runFileRestoreFromCommit(path string) {
	commits := git.RecentCommits(20)
	if len(commits) == 0 {
		ui.Info("커밋 기록이 없습니다")
		return
	}
	var sel string
	if err := survey.AskOne(&survey.Select{
		Message: "복원할 시점 선택:",
		Options: commits,
	}, &sel); err != nil {
		return
	}
	hash := strings.Fields(sel)[0]
	var ok bool
	_ = survey.AskOne(&survey.Confirm{
		Message: fmt.Sprintf("'%s' 를 [%s] 시점으로 복원할까요?", path, hash),
		Default: false,
	}, &ok)
	if !ok {
		return
	}
	if err := git.RunLive("restore", "--source="+hash, "--", path); err != nil {
		ui.Fail("복원 실패")
	} else {
		ui.Success(fmt.Sprintf("'%s' 복원 완료 ([%s] 시점)", path, hash))
	}
	fmt.Println()
}
