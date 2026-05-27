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

var resetCmd = &cobra.Command{
	Use:   "reset",
	Short: "언스테이징 / 커밋 되돌리기",
	Run: func(cmd *cobra.Command, args []string) {
		if !git.IsRepo() {
			ui.Fail("git 저장소가 아닙니다")
			os.Exit(1)
		}
		runResetMenu()
	},
}

func init() {
	rootCmd.AddCommand(resetCmd)
}

func runResetMenu() {
	lines := git.StatusShort()

	hasStagedFiles := false
	var stagedFiles []string
	for _, l := range lines {
		if len(l) < 3 {
			continue
		}
		x := l[0]
		if x != ' ' && x != '?' {
			hasStagedFiles = true
			stagedFiles = append(stagedFiles, l[3:])
		}
	}

	fmt.Println()
	if len(lines) > 0 {
		fmt.Printf("  %s\n", ui.Bold("현재 변경사항:"))
		for _, l := range lines {
			if len(l) < 3 {
				continue
			}
			fmt.Printf("    %s  %s\n", ui.ColorXY(l[:2]), l[3:])
		}
		fmt.Println()
	}

	var options []string
	if hasStagedFiles {
		options = append(options, "모든 파일 언스테이징  (git reset HEAD)")
		if len(stagedFiles) > 1 {
			options = append(options, "파일 선택해서 언스테이징")
		}
	}
	options = append(options,
		"Soft reset — 커밋 취소, 변경사항 스테이징 유지",
		"Mixed reset — 커밋 취소, 변경사항 언스테이징 (기본)",
		"Hard reset — 커밋 취소 + 변경사항 전부 폐기 ⚠",
		"취소",
	)

	var action string
	if err := survey.AskOne(&survey.Select{
		Message: "작업 선택:",
		Options: options,
	}, &action); err != nil || action == "취소" {
		return
	}

	switch {
	case strings.HasPrefix(action, "모든 파일 언스테이징"):
		if _, err := git.Run("reset", "HEAD"); err != nil {
			ui.Fail("언스테이징 실패: " + err.Error())
			return
		}
		ui.Success("모든 파일 언스테이징 완료")

	case strings.HasPrefix(action, "파일 선택해서 언스테이징"):
		runUnstageSelect(stagedFiles)

	case strings.HasPrefix(action, "Soft reset"):
		doReset("soft")

	case strings.HasPrefix(action, "Mixed reset"):
		doReset("mixed")

	case strings.HasPrefix(action, "Hard reset"):
		doHardReset()
	}
}

func runUnstageSelect(stagedFiles []string) {
	var selected []string
	if err := survey.AskOne(&survey.MultiSelect{
		Message: "언스테이징할 파일 선택:",
		Options: stagedFiles,
	}, &selected); err != nil || len(selected) == 0 {
		return
	}
	for _, f := range selected {
		if _, err := git.Run("reset", "HEAD", "--", f); err != nil {
			ui.Fail(fmt.Sprintf("언스테이징 실패 (%s): %s", f, err.Error()))
			return
		}
	}
	ui.Success(fmt.Sprintf("%d개 파일 언스테이징 완료", len(selected)))
}

func doReset(mode string) {
	var countStr string
	if err := survey.AskOne(&survey.Input{
		Message: "몇 개의 커밋을 되돌릴까요?",
		Default: "1",
	}, &countStr); err != nil {
		return
	}

	n, err := strconv.Atoi(strings.TrimSpace(countStr))
	if err != nil || n < 1 {
		ui.Fail("유효한 숫자를 입력하세요")
		return
	}

	target := fmt.Sprintf("HEAD~%d", n)
	fmt.Println()
	fmt.Printf("  %s  git reset --%s %s\n\n", ui.Bold("실행:"), mode, target)

	if _, err := git.Run("reset", "--"+mode, target); err != nil {
		ui.Fail(fmt.Sprintf("reset --%s 실패: %s", mode, err.Error()))
		return
	}
	ui.Success(fmt.Sprintf("%d개 커밋 되돌리기 완료 (mode: %s)", n, mode))

	if mode == "soft" {
		fmt.Printf("\n  %s 변경사항이 스테이징 상태로 남아있습니다\n", ui.Dim("Tip:"))
	} else {
		fmt.Printf("\n  %s 변경사항이 워킹 트리에 남아있습니다 (gez c 로 다시 커밋)\n", ui.Dim("Tip:"))
	}
	fmt.Println()
}

func doHardReset() {
	var countStr string
	if err := survey.AskOne(&survey.Input{
		Message: "몇 개의 커밋을 되돌릴까요?",
		Default: "1",
	}, &countStr); err != nil {
		return
	}

	n, err := strconv.Atoi(strings.TrimSpace(countStr))
	if err != nil || n < 1 {
		ui.Fail("유효한 숫자를 입력하세요")
		return
	}

	target := fmt.Sprintf("HEAD~%d", n)

	// Show what will be lost
	out, _ := git.Run("log", "--oneline", fmt.Sprintf("-%d", n))
	fmt.Println()
	fmt.Printf("  %s 삭제될 커밋:\n", ui.BoldRed("⚠"))
	for _, l := range strings.Split(out, "\n") {
		if l != "" {
			fmt.Printf("    %s  %s\n", ui.Red("✖"), l)
		}
	}
	fmt.Println()

	var ok bool
	if err := survey.AskOne(&survey.Confirm{
		Message: fmt.Sprintf("HEAD~%d 로 hard reset 하면 모든 변경사항이 사라집니다. 정말 계속할까요?", n),
		Default: false,
	}, &ok); err != nil || !ok {
		ui.Warn("취소되었습니다")
		return
	}

	if _, err := git.Run("reset", "--hard", target); err != nil {
		ui.Fail("hard reset 실패: " + err.Error())
		return
	}
	ui.Success(fmt.Sprintf("%d개 커밋 hard reset 완료", n))
	fmt.Println()
}
