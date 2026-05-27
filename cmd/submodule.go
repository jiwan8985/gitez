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

var submoduleCmd = &cobra.Command{
	Use:     "submodule",
	Aliases: []string{"sub"},
	Short:   "Git 서브모듈 관리 (add·update·sync·status·foreach)",
	Run: func(cmd *cobra.Command, args []string) {
		if !git.IsRepo() {
			ui.Fail("git 저장소가 아닙니다")
			os.Exit(1)
		}
		runSubmoduleMenu()
	},
}

var submoduleAddCmd = &cobra.Command{
	Use:   "add <url> [경로]",
	Short: "서브모듈 추가",
	Args:  cobra.RangeArgs(1, 2),
	Run: func(cmd *cobra.Command, args []string) {
		url := args[0]
		path := ""
		if len(args) > 1 {
			path = args[1]
		}
		doSubmoduleAdd(url, path)
	},
}

var submoduleUpdateCmd = &cobra.Command{
	Use:   "update",
	Short: "서브모듈 업데이트 (초기화 포함)",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println()
		ui.Info("서브모듈 업데이트 중…")
		if err := git.RunLive("submodule", "update", "--init", "--recursive"); err != nil {
			ui.Fail("업데이트 실패")
		} else {
			ui.Success("서브모듈 업데이트 완료")
		}
		fmt.Println()
	},
}

var submoduleSyncCmd = &cobra.Command{
	Use:   "sync",
	Short: "서브모듈 URL 동기화",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println()
		if err := git.RunLive("submodule", "sync", "--recursive"); err != nil {
			ui.Fail("sync 실패")
		} else {
			ui.Success("서브모듈 URL 동기화 완료")
		}
		fmt.Println()
	},
}

var submoduleStatusCmd = &cobra.Command{
	Use:     "status",
	Aliases: []string{"st"},
	Short:   "서브모듈 상태 보기",
	Run: func(cmd *cobra.Command, args []string) {
		printSubmoduleStatus()
	},
}

var submoduleForeachCmd = &cobra.Command{
	Use:   "foreach <명령어>",
	Short: "모든 서브모듈에서 명령어 실행",
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		command := strings.Join(args, " ")
		fmt.Println()
		ui.Info(fmt.Sprintf("서브모듈 foreach: %s", command))
		fmt.Println()
		if err := git.RunLive("submodule", "foreach", "--recursive", command); err != nil {
			ui.Fail("foreach 실패")
		}
		fmt.Println()
	},
}

func init() {
	submoduleCmd.AddCommand(submoduleAddCmd)
	submoduleCmd.AddCommand(submoduleUpdateCmd)
	submoduleCmd.AddCommand(submoduleSyncCmd)
	submoduleCmd.AddCommand(submoduleStatusCmd)
	submoduleCmd.AddCommand(submoduleForeachCmd)
	rootCmd.AddCommand(submoduleCmd)
}

// ── 대화형 메뉴 ───────────────────────────────────────────────────────────────

func runSubmoduleMenu() {
	printSubmoduleStatus()

	mods := git.SubmoduleList()
	actions := []string{"서브모듈 추가 (add)"}
	if len(mods) > 0 {
		actions = append(actions,
			"전체 업데이트 (update --init --recursive)",
			"URL 동기화 (sync)",
			"각 서브모듈에서 명령 실행 (foreach)",
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
	case strings.HasPrefix(action, "서브모듈 추가"):
		var url string
		if err := survey.AskOne(&survey.Input{
			Message: "서브모듈 URL:",
		}, &url, survey.WithValidator(survey.Required)); err != nil {
			return
		}
		var path string
		_ = survey.AskOne(&survey.Input{
			Message: "로컬 경로 (비워두면 자동):",
		}, &path)
		doSubmoduleAdd(strings.TrimSpace(url), strings.TrimSpace(path))

	case strings.HasPrefix(action, "전체 업데이트"):
		ui.Info("서브모듈 업데이트 중…")
		if err := git.RunLive("submodule", "update", "--init", "--recursive"); err != nil {
			ui.Fail("업데이트 실패")
		} else {
			ui.Success("완료")
		}

	case strings.HasPrefix(action, "URL 동기화"):
		if err := git.RunLive("submodule", "sync", "--recursive"); err != nil {
			ui.Fail("sync 실패")
		} else {
			ui.Success("완료")
		}

	case strings.HasPrefix(action, "각 서브모듈"):
		var cmd string
		if err := survey.AskOne(&survey.Input{
			Message: "실행할 명령어:",
			Help:    "예: git pull origin main",
		}, &cmd, survey.WithValidator(survey.Required)); err != nil {
			return
		}
		_ = git.RunLive("submodule", "foreach", "--recursive", cmd)
	}
	fmt.Println()
}

// ── 실행 로직 ─────────────────────────────────────────────────────────────────

func printSubmoduleStatus() {
	mods := git.SubmoduleList()
	fmt.Println()
	if len(mods) == 0 {
		fmt.Printf("  %s\n\n", ui.Dim("서브모듈 없음"))
		return
	}

	fmt.Printf("  %s  (%d개)\n\n", ui.Bold("서브모듈 목록"), len(mods))
	for _, m := range mods {
		statusIcon := ui.Dim("  ")
		switch m.Status {
		case "+":
			statusIcon = ui.Yellow("⬆ ") // newer commit checked out
		case "-":
			statusIcon = ui.Red("✗ ") // not initialized
		case "U":
			statusIcon = ui.Red("⚡") // conflict
		default:
			statusIcon = ui.Green("✔ ")
		}
		desc := ""
		if m.Desc != "" {
			desc = ui.Dim("  " + m.Desc)
		}
		fmt.Printf("  %s %s  %s%s\n", statusIcon, ui.Cyan(m.Path), ui.Yellow(m.Hash[:minLen(m.Hash, 8)]), desc)
	}
	fmt.Println()
}

func doSubmoduleAdd(url, path string) {
	args := []string{"submodule", "add", url}
	if path != "" {
		args = append(args, path)
	}
	fmt.Println()
	if err := git.RunLive(args...); err != nil {
		ui.Fail("서브모듈 추가 실패")
		return
	}
	ui.Success("서브모듈 추가 완료!")
	fmt.Printf("  %s  git submodule 설정이 .gitmodules 에 저장됐습니다\n\n", ui.Dim("Tip:"))
}

func minLen(s string, n int) int {
	if len(s) < n {
		return len(s)
	}
	return n
}
