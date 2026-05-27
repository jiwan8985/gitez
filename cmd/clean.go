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

var cleanCmd = &cobra.Command{
	Use:   "clean",
	Short: "추적되지 않는 파일·디렉토리 정리",
	Run: func(cmd *cobra.Command, args []string) {
		if !git.IsRepo() {
			ui.Fail("git 저장소가 아닙니다")
			os.Exit(1)
		}
		runClean()
	},
}

func init() {
	rootCmd.AddCommand(cleanCmd)
}

func runClean() {
	// Dry-run으로 삭제 예정 파일 파악
	dryOut, err := git.Run("clean", "-n", "-d")
	if err != nil || strings.TrimSpace(dryOut) == "" {
		ui.Info("정리할 untracked 파일이 없습니다")
		return
	}

	fmt.Println()
	fmt.Printf("  %s\n", ui.Bold("삭제 예정 파일 (dry-run):"))
	lines := strings.Split(strings.TrimSpace(dryOut), "\n")
	for _, l := range lines {
		l = strings.TrimPrefix(l, "Would remove ")
		l = strings.TrimPrefix(l, "Would skip ")
		if l != "" {
			fmt.Printf("    %s  %s\n", ui.Red("✖"), l)
		}
	}
	fmt.Println()

	// 모드 선택
	var mode string
	if err := survey.AskOne(&survey.Select{
		Message: "정리 범위:",
		Options: []string{
			"untracked 파일만 삭제",
			"untracked 파일 + 빈 디렉토리 삭제  (-d)",
			"untracked + .gitignore 파일까지  (-d -x)  ⚠",
			"파일 직접 선택해서 삭제",
			"취소",
		},
	}, &mode); err != nil || mode == "취소" {
		return
	}

	switch {
	case strings.HasPrefix(mode, "untracked 파일만"):
		confirmAndClean([]string{"clean", "-f"})

	case strings.HasPrefix(mode, "untracked 파일 + 빈"):
		confirmAndClean([]string{"clean", "-f", "-d"})

	case strings.HasPrefix(mode, "untracked + .gitignore"):
		ui.Warn(".gitignore 파일(빌드 결과물, 캐시 등)도 삭제됩니다")
		fmt.Println()
		var ok bool
		if err := survey.AskOne(&survey.Confirm{
			Message: "정말 .gitignore 파일까지 삭제할까요?",
			Default: false,
		}, &ok); err != nil || !ok {
			ui.Warn("취소되었습니다")
			return
		}
		confirmAndClean([]string{"clean", "-f", "-d", "-x"})

	case strings.HasPrefix(mode, "파일 직접 선택"):
		runCleanSelect()
	}
}

func confirmAndClean(args []string) {
	var ok bool
	if err := survey.AskOne(&survey.Confirm{
		Message: "위 파일들을 삭제할까요? (복구 불가)",
		Default: false,
	}, &ok); err != nil || !ok {
		ui.Warn("취소되었습니다")
		return
	}

	fmt.Println()
	if err := git.RunLive(args...); err != nil {
		ui.Fail("clean 실패")
		return
	}
	fmt.Println()
	ui.Success("정리 완료!")
	fmt.Println()
}

func runCleanSelect() {
	// untracked 파일 목록 (디렉토리 포함)
	out, err := git.Run("clean", "-n", "-d")
	if err != nil || out == "" {
		ui.Info("정리할 파일이 없습니다")
		return
	}

	var fileOptions []string
	for _, l := range strings.Split(out, "\n") {
		l = strings.TrimPrefix(l, "Would remove ")
		l = strings.TrimPrefix(l, "Would skip ")
		l = strings.TrimSpace(l)
		if l != "" {
			fileOptions = append(fileOptions, l)
		}
	}

	if len(fileOptions) == 0 {
		ui.Info("선택 가능한 파일이 없습니다")
		return
	}

	var selected []string
	if err := survey.AskOne(&survey.MultiSelect{
		Message: "삭제할 파일/디렉토리 선택 (Space=선택, Enter=확인):",
		Options: fileOptions,
	}, &selected); err != nil || len(selected) == 0 {
		ui.Warn("선택된 항목이 없습니다")
		return
	}

	fmt.Println()
	fmt.Printf("  %s\n", ui.Bold("삭제 대상:"))
	for _, s := range selected {
		fmt.Printf("    %s  %s\n", ui.Red("✖"), s)
	}
	fmt.Println()

	var ok bool
	if err := survey.AskOne(&survey.Confirm{
		Message: fmt.Sprintf("%d개 항목을 삭제할까요? (복구 불가)", len(selected)),
		Default: false,
	}, &ok); err != nil || !ok {
		ui.Warn("취소되었습니다")
		return
	}

	// 선택 항목을 개별 삭제
	for _, item := range selected {
		item = strings.TrimSuffix(item, "/")
		if err := os.RemoveAll(item); err != nil {
			ui.Warn(fmt.Sprintf("삭제 실패 (%s): %s", item, err.Error()))
		}
	}

	ui.Success(fmt.Sprintf("%d개 항목 삭제 완료!", len(selected)))
	fmt.Println()
}
