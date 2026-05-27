package cmd

import (
	"fmt"
	"gez/internal/git"
	"gez/internal/ui"
	"os"

	"github.com/spf13/cobra"
)

var fetchCmd = &cobra.Command{
	Use:     "fetch",
	Aliases: []string{"f"},
	Short:   "원격 저장소 정보 가져오기 (fetch --all --prune)",
	Run: func(cmd *cobra.Command, args []string) {
		if !git.IsRepo() {
			ui.Fail("git 저장소가 아닙니다")
			os.Exit(1)
		}
		runFetch()
	},
}

func init() {
	rootCmd.AddCommand(fetchCmd)
}

func runFetch() {
	fmt.Printf("\n  %s\n\n", ui.Bold("Fetching origin (--all --prune)..."))
	if err := git.RunLive("fetch", "--all", "--prune"); err != nil {
		ui.Fail("페치 실패")
		return
	}
	fmt.Println()
	ui.Success("페치 완료!")
	fmt.Println()
}
