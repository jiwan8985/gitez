package cmd

import (
	"fmt"
	"gez/internal/git"
	"gez/internal/ui"
	"gez/internal/workspace"
	"os"
	"path/filepath"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"
)

var cloneCmd = &cobra.Command{
	Use:   "clone <URL> [디렉토리]",
	Short: "저장소 클론 (+ 워크스페이스 자동 등록)",
	Args:  cobra.RangeArgs(1, 2),
	Run: func(cmd *cobra.Command, args []string) {
		url := args[0]
		dir := ""
		if len(args) > 1 {
			dir = args[1]
		} else {
			// Derive directory name from URL
			dir = guessCloneDir(url)
		}
		runClone(url, dir)
	},
}

func init() {
	rootCmd.AddCommand(cloneCmd)
}

func runClone(url, dir string) {
	fmt.Println()
	fmt.Printf("  %s\n", ui.Bold(fmt.Sprintf("Cloning  %s", url)))
	fmt.Printf("  %s  %s\n\n", ui.Dim("→"), ui.Cyan(dir))

	var cloneArgs []string
	if dir != "" {
		cloneArgs = []string{"clone", url, dir}
	} else {
		cloneArgs = []string{"clone", url}
	}

	if err := git.RunLive(cloneArgs...); err != nil {
		ui.Fail("클론 실패")
		return
	}

	abs, err := filepath.Abs(dir)
	if err != nil {
		ui.Fail("경로 오류: " + err.Error())
		return
	}

	if _, statErr := os.Stat(abs); os.IsNotExist(statErr) {
		ui.Warn("클론된 폴더를 찾을 수 없습니다: " + abs)
		return
	}

	fmt.Println()
	ui.Success(fmt.Sprintf("클론 완료!  →  %s", workspace.HomePath(abs)))

	// Offer workspace registration
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

// guessCloneDir extracts a directory name from a git clone URL.
func guessCloneDir(url string) string {
	// Strip trailing slashes and .git suffix
	url = strings.TrimRight(url, "/")
	base := filepath.Base(url)
	base = strings.TrimSuffix(base, ".git")
	if base == "" || base == "." {
		return "repo"
	}
	return base
}
