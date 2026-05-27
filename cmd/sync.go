package cmd

import (
	"fmt"
	"gez/internal/git"
	"gez/internal/ui"
	"os"

	"github.com/spf13/cobra"
)

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "fetch + pull (원격과 동기화)",
	Run: func(cmd *cobra.Command, args []string) {
		if !git.IsRepo() {
			ui.Fail("git 저장소가 아닙니다")
			os.Exit(1)
		}
		runSync()
	},
}

func init() {
	rootCmd.AddCommand(syncCmd)
}

func runSync() {
	branch := git.CurrentBranch()
	fmt.Printf("\n  %s  [%s]\n\n", ui.Bold("원격 동기화"), ui.Cyan(branch))

	fmt.Printf("  %s fetch --all --prune\n", ui.Dim("1."))
	if err := git.RunLive("fetch", "--all", "--prune"); err != nil {
		ui.Fail("페치 실패")
		return
	}

	fmt.Printf("\n  %s pull\n", ui.Dim("2."))
	if err := git.RunLive("pull"); err != nil {
		ui.Warn("풀 실패 — 충돌 또는 upstream 없음")
		return
	}

	fmt.Println()
	ui.Success("동기화 완료!")
	fmt.Println()
}
