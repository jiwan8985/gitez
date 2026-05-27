package cmd

import (
	"fmt"
	"gez/internal/git"
	"gez/internal/ui"
	"gez/internal/workspace"
	"os"
	"path/filepath"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init [경로]",
	Short: "새 git 저장소 초기화 (+ 워크스페이스 등록)",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		path := "."
		if len(args) > 0 {
			path = args[0]
		}
		runInit(path)
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
}

func runInit(path string) {
	abs, err := filepath.Abs(path)
	if err != nil {
		ui.Fail("경로 오류: " + err.Error())
		return
	}

	// Create directory if it doesn't exist
	if err := os.MkdirAll(abs, 0755); err != nil {
		ui.Fail("디렉토리 생성 실패: " + err.Error())
		return
	}

	fmt.Println()
	fmt.Printf("  %s  %s\n\n", ui.Bold("git init:"), ui.Cyan(workspace.HomePath(abs)))

	if err := git.RunLiveInDir(abs, "init"); err != nil {
		ui.Fail("git init 실패")
		return
	}

	fmt.Println()
	ui.Success(fmt.Sprintf("git 저장소 초기화 완료  →  %s", workspace.HomePath(abs)))

	// Offer to add to workspace
	var addToWs bool
	if err := survey.AskOne(&survey.Confirm{
		Message: "워크스페이스에 등록할까요?",
		Default: true,
	}, &addToWs); err == nil && addToWs {
		ws, err := workspace.Load()
		if err != nil {
			ui.Warn("워크스페이스 로드 실패: " + err.Error())
			return
		}
		if err := ws.Add(abs); err != nil {
			ui.Warn(err.Error())
			return
		}
		proj := ws.Projects[len(ws.Projects)-1]
		ui.Success(fmt.Sprintf("워크스페이스에 '%s' 등록 완료", proj.Name))
		fmt.Printf("  %s  %s\n", ui.Dim("이제 이렇게 사용하세요:"), ui.Cyan(fmt.Sprintf("gez -p %s <명령어>", proj.Name)))
	}

	fmt.Println()
}
