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

var blameCmd = &cobra.Command{
	Use:   "blame [파일]",
	Short: "파일의 줄별 작성자·커밋 보기",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if !git.IsRepo() {
			ui.Fail("git 저장소가 아닙니다")
			os.Exit(1)
		}
		if len(args) > 0 {
			doBlame(args[0])
		} else {
			runBlameInteractive()
		}
	},
}

func init() {
	rootCmd.AddCommand(blameCmd)
}

// ── 파일 선택 ─────────────────────────────────────────────────────────────────

func runBlameInteractive() {
	// 트래킹 중인 파일 + 변경된 파일 목록
	tracked := git.TrackedFiles()
	if len(tracked) == 0 {
		ui.Info("트래킹 중인 파일이 없습니다")
		return
	}

	// 변경된 파일은 상단에 표시
	changed := map[string]bool{}
	for _, l := range git.StatusShort() {
		if len(l) >= 3 {
			changed[strings.TrimSpace(l[3:])] = true
		}
	}

	var options []string
	for _, f := range tracked {
		if changed[f] {
			options = append([]string{fmt.Sprintf("%s  %s", f, ui.Yellow("(변경됨)"))}, options...)
		} else {
			options = append(options, f)
		}
	}

	fmt.Println()
	var selected string
	if err := survey.AskOne(&survey.Select{
		Message: "blame 볼 파일 선택:",
		Options: options,
	}, &selected); err != nil {
		return
	}

	// 파일 이름만 추출 (색상 태그 제거)
	file := strings.Fields(selected)[0]
	doBlame(file)
}

// ── 실행 ─────────────────────────────────────────────────────────────────────

func doBlame(file string) {
	if _, err := os.Stat(file); os.IsNotExist(err) {
		ui.Fail(fmt.Sprintf("파일을 찾을 수 없습니다: %s", file))
		return
	}

	fmt.Println()
	fmt.Printf("  %s  %s\n\n", ui.Bold("Blame:"), ui.Cyan(file))

	// --color-lines: 같은 커밋의 줄을 같은 색으로
	// --date=short: 날짜 짧게
	// -w: 공백 변경 무시
	err := git.RunLive(
		"blame",
		"--color-lines",
		"--date=short",
		"-w",
		file,
	)
	if err != nil {
		// color-lines 미지원 버전 대비 fallback
		_ = git.RunLive("blame", "--date=short", "-w", file)
	}
	fmt.Println()
}
