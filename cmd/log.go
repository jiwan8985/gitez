package cmd

import (
	"fmt"
	"gez/internal/git"
	"gez/internal/ui"
	"os"

	"github.com/spf13/cobra"
)

var logCount int

var logCmd = &cobra.Command{
	Use:     "log",
	Aliases: []string{"l"},
	Short:   "커밋 그래프 로그 보기",
	Run: func(cmd *cobra.Command, args []string) {
		if !git.IsRepo() {
			ui.Fail("git 저장소가 아닙니다")
			os.Exit(1)
		}
		runLog()
	},
}

func init() {
	logCmd.Flags().IntVarP(&logCount, "count", "n", 20, "표시할 커밋 수")
	rootCmd.AddCommand(logCmd)
}

func runLog() {
	// Pretty one-line graph log using git's own color codes (--color=always)
	format := "%C(yellow)%h%C(reset)  %C(cyan)%ad%C(reset)  %C(bold white)%s%C(reset)  %C(dim)%an%C(reset)"
	fmt.Println()
	err := git.RunLive(
		"log",
		"--graph",
		"--color=always",
		fmt.Sprintf("--pretty=format:%s", format),
		"--date=short",
		fmt.Sprintf("-n%d", logCount),
		"--decorate",
	)
	if err != nil {
		ui.Fail("로그 조회 실패")
		return
	}
	fmt.Println()
	fmt.Println()
}
