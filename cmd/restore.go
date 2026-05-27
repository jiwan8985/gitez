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

var restoreCmd = &cobra.Command{
	Use:   "restore [파일...]",
	Short: "파일을 HEAD(또는 지정 커밋) 상태로 복원",
	Run: func(cmd *cobra.Command, args []string) {
		if !git.IsRepo() {
			ui.Fail("git 저장소가 아닙니다")
			os.Exit(1)
		}
		runRestore(args)
	},
}

func init() {
	rootCmd.AddCommand(restoreCmd)
}

func runRestore(args []string) {
	lines := git.StatusShort()
	if len(lines) == 0 {
		ui.Info("복원할 변경사항이 없습니다")
		return
	}

	// Build modified/deleted file list
	var modFiles []string
	var stagedFiles []string
	for _, l := range lines {
		if len(l) < 3 {
			continue
		}
		x, y := l[0], l[1]
		path := strings.TrimSpace(l[3:])
		if y != ' ' && y != '?' {
			modFiles = append(modFiles, path)
		}
		if x != ' ' && x != '?' {
			stagedFiles = append(stagedFiles, path)
		}
	}

	fmt.Println()
	fmt.Println(ui.Bold("  현재 변경사항:"))
	for _, l := range lines {
		if len(l) < 3 {
			continue
		}
		fmt.Printf("    %s  %s\n", ui.ColorXY(l[:2]), l[3:])
	}
	fmt.Println()

	// If specific files given, restore them directly
	if len(args) > 0 {
		doRestoreFiles(args, "HEAD")
		return
	}

	var action string
	if err := survey.AskOne(&survey.Select{
		Message: "복원 방법:",
		Options: []string{
			"워킹 트리 변경사항 되돌리기 (unstaged만)",
			"스테이징 취소 (staged → unstaged)",
			"특정 커밋 시점으로 파일 복원",
			"전체 되돌리기 (staged + unstaged)",
			"취소",
		},
	}, &action); err != nil || action == "취소" {
		return
	}

	fmt.Println()
	switch {
	case strings.HasPrefix(action, "워킹 트리"):
		if len(modFiles) == 0 {
			ui.Info("unstaged 변경사항이 없습니다")
			return
		}
		var selected []string
		if err := survey.AskOne(&survey.MultiSelect{
			Message: "복원할 파일 선택:",
			Options: modFiles,
		}, &selected); err != nil || len(selected) == 0 {
			return
		}
		doRestoreFiles(selected, "")

	case strings.HasPrefix(action, "스테이징 취소"):
		if len(stagedFiles) == 0 {
			ui.Info("staged 파일이 없습니다")
			return
		}
		var selected []string
		if err := survey.AskOne(&survey.MultiSelect{
			Message: "unstage할 파일 선택:",
			Options: stagedFiles,
		}, &selected); err != nil || len(selected) == 0 {
			return
		}
		args := append([]string{"restore", "--staged"}, selected...)
		if err := git.RunLive(args...); err != nil {
			ui.Fail("unstage 실패")
		} else {
			ui.Success(fmt.Sprintf("%d개 파일 unstaged", len(selected)))
		}

	case strings.HasPrefix(action, "특정 커밋"):
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
		sourceHash := strings.Fields(sel)[0]

		allFiles := append(modFiles, stagedFiles...)
		dedup := make(map[string]bool)
		var unique []string
		for _, f := range allFiles {
			if !dedup[f] {
				dedup[f] = true
				unique = append(unique, f)
			}
		}
		var selected []string
		if err := survey.AskOne(&survey.MultiSelect{
			Message: "복원할 파일 선택:",
			Options: unique,
		}, &selected); err != nil || len(selected) == 0 {
			return
		}
		doRestoreFiles(selected, sourceHash)

	case strings.HasPrefix(action, "전체 되돌리기"):
		var ok bool
		_ = survey.AskOne(&survey.Confirm{
			Message: "모든 변경사항(staged + unstaged)을 HEAD로 되돌릴까요?",
			Default: false,
		}, &ok)
		if !ok {
			return
		}
		_, _ = git.Run("restore", "--staged", ".")
		if err := git.RunLive("restore", "."); err != nil {
			ui.Fail("복원 실패")
		} else {
			ui.Success("전체 복원 완료")
		}
	}
	fmt.Println()
}

func doRestoreFiles(files []string, source string) {
	args := []string{"restore"}
	if source != "" {
		args = append(args, "--source="+source)
	}
	args = append(args, "--")
	args = append(args, files...)

	if err := git.RunLive(args...); err != nil {
		ui.Fail("복원 실패: " + err.Error())
		return
	}
	ui.Success(fmt.Sprintf("%d개 파일 복원 완료", len(files)))
}
