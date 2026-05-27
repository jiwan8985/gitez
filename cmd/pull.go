package cmd

import (
	"fmt"
	"gez/internal/git"
	"gez/internal/ui"
	"os"

	"github.com/spf13/cobra"
)

var pullCmd = &cobra.Command{
	Use:   "pull",
	Short: "원격 저장소에서 풀",
	Run: func(cmd *cobra.Command, args []string) {
		if !git.IsRepo() {
			ui.Fail("git 저장소가 아닙니다")
			os.Exit(1)
		}
		runPull()
	},
}

func init() {
	rootCmd.AddCommand(pullCmd)
}

func runPull() {
	branch := git.CurrentBranch()
	fmt.Printf("\n  %s  origin/%s → %s\n\n",
		ui.Bold("Pull:"), ui.Cyan(branch), ui.Cyan(branch))

	if err := git.RunLive("pull"); err != nil {
		ui.Fail("풀 실패 (충돌이 있을 수 있습니다)")
		return
	}

	fmt.Println()
	ui.Success("풀 완료!")
	fmt.Println()
}
