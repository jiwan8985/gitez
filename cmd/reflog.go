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

var reflogCount int

var reflogCmd = &cobra.Command{
	Use:   "reflog",
	Short: "reflog 조회 및 커밋 복구",
	Run: func(cmd *cobra.Command, args []string) {
		if !git.IsRepo() {
			ui.Fail("git 저장소가 아닙니다")
			os.Exit(1)
		}
		runReflog()
	},
}

func init() {
	reflogCmd.Flags().IntVarP(&reflogCount, "count", "n", 30, "표시할 reflog 항목 수")
	rootCmd.AddCommand(reflogCmd)
}

func runReflog() {
	entries := git.ReflogEntries(reflogCount)
	if len(entries) == 0 {
		ui.Info("reflog 항목이 없습니다")
		return
	}

	fmt.Println()
	fmt.Printf("  %s  %s\n\n",
		ui.Bold("Reflog"),
		ui.Dim(fmt.Sprintf("(최근 %d개 — 사라진 커밋 복구에 사용)", len(entries))))

	// 항목 출력 (색상)
	for i, e := range entries {
		parts := strings.SplitN(e, " ", 2)
		ref := ""
		rest := e
		if len(parts) == 2 {
			ref = parts[0]
			rest = parts[1]
		}
		fmt.Printf("  %s  %s  %s\n",
			ui.Dim(fmt.Sprintf("%3d", i+1)),
			ui.Yellow(ref),
			rest)
	}
	fmt.Println()

	// 복구 액션 여부
	var doRestore bool
	if err := survey.AskOne(&survey.Confirm{
		Message: "선택한 항목으로 복구할까요?",
		Default: false,
	}, &doRestore); err != nil || !doRestore {
		return
	}

	var selected string
	if err := survey.AskOne(&survey.Select{
		Message: "복구할 reflog 항목 선택:",
		Options: entries,
	}, &selected); err != nil {
		return
	}

	// 해시 추출 (첫 번째 필드)
	hash := strings.Fields(selected)[0]

	fmt.Println()
	var method string
	if err := survey.AskOne(&survey.Select{
		Message: "복구 방법:",
		Options: []string{
			fmt.Sprintf("새 브랜치 생성  — checkout -b recover/%s %s", hash[:7], hash),
			fmt.Sprintf("Soft reset  — 변경사항 스테이징 유지  (reset --soft %s)", hash),
			fmt.Sprintf("Mixed reset  — 변경사항 언스테이징  (reset --mixed %s)", hash),
			fmt.Sprintf("Hard reset  — 모든 변경사항 폐기 ⚠  (reset --hard %s)", hash),
		},
	}, &method); err != nil {
		return
	}

	fmt.Println()
	switch {
	case strings.HasPrefix(method, "새 브랜치"):
		branchName := "recover/" + hash[:7]
		if _, err := git.Run("checkout", "-b", branchName, hash); err != nil {
			ui.Fail("브랜치 생성 실패: " + err.Error())
			return
		}
		ui.Success(fmt.Sprintf("브랜치 '%s' 생성 완료 — 복구된 커밋이 여기 있습니다", branchName))

	case strings.HasPrefix(method, "Soft"):
		if _, err := git.Run("reset", "--soft", hash); err != nil {
			ui.Fail("reset 실패: " + err.Error())
			return
		}
		ui.Success(fmt.Sprintf("Soft reset → %s 완료 (변경사항 스테이징됨)", hash[:7]))

	case strings.HasPrefix(method, "Mixed"):
		if _, err := git.Run("reset", "--mixed", hash); err != nil {
			ui.Fail("reset 실패: " + err.Error())
			return
		}
		ui.Success(fmt.Sprintf("Mixed reset → %s 완료 (변경사항 언스테이징됨)", hash[:7]))

	case strings.HasPrefix(method, "Hard"):
		var confirm bool
		if err := survey.AskOne(&survey.Confirm{
			Message: fmt.Sprintf("⚠  hard reset → %s 하면 모든 변경사항이 사라집니다. 계속할까요?", hash[:7]),
			Default: false,
		}, &confirm); err != nil || !confirm {
			ui.Warn("취소되었습니다")
			return
		}
		if _, err := git.Run("reset", "--hard", hash); err != nil {
			ui.Fail("reset 실패: " + err.Error())
			return
		}
		ui.Success(fmt.Sprintf("Hard reset → %s 완료", hash[:7]))
	}
	fmt.Println()
}
