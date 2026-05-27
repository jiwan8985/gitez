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

var sparseCmd = &cobra.Command{
	Use:     "sparse",
	Aliases: []string{"sparse-checkout"},
	Short:   "Sparse checkout 관리 — 모노레포에서 일부 디렉토리만 체크아웃",
	Run: func(cmd *cobra.Command, args []string) {
		if !git.IsRepo() {
			ui.Fail("git 저장소가 아닙니다")
			os.Exit(1)
		}
		runSparseMenu()
	},
}

var sparseInitCmd = &cobra.Command{
	Use:   "init",
	Short: "sparse-checkout 활성화",
	Run: func(cmd *cobra.Command, args []string) {
		doSparseInit()
	},
}

var sparseAddCmd = &cobra.Command{
	Use:   "add <경로...>",
	Short: "sparse 패턴 추가",
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		doSparseAdd(args)
	},
}

var sparseSetCmd = &cobra.Command{
	Use:   "set <경로...>",
	Short: "sparse 패턴 새로 설정 (기존 패턴 대체)",
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		doSparseSet(args)
	},
}

var sparseListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "현재 sparse-checkout 패턴 보기",
	Run:     func(cmd *cobra.Command, args []string) { printSparseList() },
}

var sparseDisableCmd = &cobra.Command{
	Use:   "disable",
	Short: "sparse-checkout 비활성화 (전체 체크아웃으로 복귀)",
	Run: func(cmd *cobra.Command, args []string) {
		doSparseDisable()
	},
}

func init() {
	sparseCmd.AddCommand(sparseInitCmd)
	sparseCmd.AddCommand(sparseAddCmd)
	sparseCmd.AddCommand(sparseSetCmd)
	sparseCmd.AddCommand(sparseListCmd)
	sparseCmd.AddCommand(sparseDisableCmd)
	rootCmd.AddCommand(sparseCmd)
}

func runSparseMenu() {
	// Check if sparse is enabled
	out, _ := git.Run("sparse-checkout", "list")
	enabled := strings.TrimSpace(out) != ""

	fmt.Println()
	if enabled {
		fmt.Printf("  %s  %s\n\n", ui.Bold("Sparse Checkout:"), ui.Green("활성화됨"))
		printSparseList()
	} else {
		fmt.Printf("  %s  %s\n\n", ui.Bold("Sparse Checkout:"), ui.Dim("비활성화됨"))
		fmt.Printf("  %s\n", ui.Dim("sparse-checkout을 사용하면 저장소의 일부만 체크아웃할 수 있습니다."))
		fmt.Printf("  %s\n\n", ui.Dim("모노레포에서 특정 패키지나 디렉토리만 필요할 때 유용합니다."))
	}

	actions := []string{}
	if !enabled {
		actions = append(actions, "sparse-checkout 활성화")
	} else {
		actions = append(actions,
			"패턴 추가",
			"패턴 새로 설정 (전체 교체)",
			"sparse-checkout 비활성화 (전체 체크아웃 복귀)",
		)
	}
	actions = append(actions, "취소")

	var action string
	if err := survey.AskOne(&survey.Select{
		Message: "작업 선택:",
		Options: actions,
	}, &action); err != nil || action == "취소" {
		return
	}

	fmt.Println()
	switch {
	case strings.HasPrefix(action, "sparse-checkout 활성화"):
		doSparseInit()

	case strings.HasPrefix(action, "패턴 추가"):
		var input string
		if err := survey.AskOne(&survey.Input{
			Message: "추가할 경로 패턴:",
			Help:    "예: packages/frontend   apps/   !docs/",
		}, &input, survey.WithValidator(survey.Required)); err != nil {
			return
		}
		doSparseAdd(strings.Fields(input))

	case strings.HasPrefix(action, "패턴 새로 설정"):
		var input string
		if err := survey.AskOne(&survey.Input{
			Message: "새 패턴 (공백으로 구분):",
			Help:    "예: packages/frontend packages/shared",
		}, &input, survey.WithValidator(survey.Required)); err != nil {
			return
		}
		doSparseSet(strings.Fields(input))

	case strings.HasPrefix(action, "sparse-checkout 비활성화"):
		doSparseDisable()
	}
}

func doSparseInit() {
	fmt.Println()

	// Choose cone mode (recommended) or full pattern
	var mode string
	if err := survey.AskOne(&survey.Select{
		Message: "모드 선택:",
		Options: []string{
			"cone 모드 — 디렉토리 기준 (권장, 빠름)",
			"no-cone 모드 — 전체 패턴 매칭 (느림, 복잡)",
		},
	}, &mode); err != nil {
		return
	}

	args := []string{"sparse-checkout", "init"}
	if strings.HasPrefix(mode, "cone") {
		args = append(args, "--cone")
	}

	if err := git.RunLive(args...); err != nil {
		ui.Fail("sparse-checkout 초기화 실패")
		return
	}
	ui.Success("sparse-checkout 활성화!")
	fmt.Println()

	// Ask for initial patterns
	var patterns string
	if err := survey.AskOne(&survey.Input{
		Message: "체크아웃할 경로 (공백으로 구분, 비우면 나중에 설정):",
		Help:    "예: packages/frontend packages/shared",
	}, &patterns); err != nil {
		return
	}
	patterns = strings.TrimSpace(patterns)
	if patterns != "" {
		doSparseSet(strings.Fields(patterns))
	}
}

func doSparseAdd(patterns []string) {
	args := append([]string{"sparse-checkout", "add"}, patterns...)
	if err := git.RunLive(args...); err != nil {
		ui.Fail("패턴 추가 실패")
		return
	}
	ui.Success(fmt.Sprintf("패턴 추가 완료: %s", strings.Join(patterns, ", ")))
	fmt.Println()
}

func doSparseSet(patterns []string) {
	args := append([]string{"sparse-checkout", "set"}, patterns...)
	if err := git.RunLive(args...); err != nil {
		ui.Fail("패턴 설정 실패")
		return
	}
	ui.Success(fmt.Sprintf("패턴 설정 완료: %s", strings.Join(patterns, ", ")))
	fmt.Println()
}

func printSparseList() {
	out, err := git.Run("sparse-checkout", "list")
	if err != nil || strings.TrimSpace(out) == "" {
		fmt.Printf("  %s\n\n", ui.Dim("(패턴 없음)"))
		return
	}
	fmt.Printf("  %s\n\n", ui.Bold("현재 sparse-checkout 패턴:"))
	for _, l := range strings.Split(strings.TrimSpace(out), "\n") {
		fmt.Printf("  %s  %s\n", ui.Cyan("·"), l)
	}
	fmt.Println()
}

func doSparseDisable() {
	var ok bool
	_ = survey.AskOne(&survey.Confirm{
		Message: "sparse-checkout을 비활성화하고 전체 파일을 체크아웃할까요?",
		Default: false,
	}, &ok)
	if !ok {
		return
	}
	fmt.Println()
	if err := git.RunLive("sparse-checkout", "disable"); err != nil {
		ui.Fail("비활성화 실패")
		return
	}
	ui.Success("전체 체크아웃으로 복귀했습니다")
	fmt.Println()
}
