package cmd

import (
	"fmt"
	"gez/internal/git"
	"gez/internal/ui"
	"os"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"
)

var pushForce bool

var pushCmd = &cobra.Command{
	Use:     "push",
	Aliases: []string{"p"},
	Short:   "원격 저장소에 푸시  [-f: force-with-lease]",
	Run: func(cmd *cobra.Command, args []string) {
		if !git.IsRepo() {
			ui.Fail("git 저장소가 아닙니다")
			os.Exit(1)
		}
		branch := git.CurrentBranch()
		doPushBranch(branch, pushForce)
	},
}

func init() {
	pushCmd.Flags().BoolVarP(&pushForce, "force", "f", false, "force-with-lease 로 강제 푸시")
	rootCmd.AddCommand(pushCmd)
}

// doPushBranch is shared by pushCmd and commitCmd.
func doPushBranch(branch string, force bool) {
	fmt.Printf("\n  %s  %s → origin/%s\n\n",
		ui.Bold("Push:"), ui.Cyan(branch), ui.Cyan(branch))

	if force {
		var ok bool
		_ = survey.AskOne(&survey.Confirm{
			Message: "force-with-lease 푸시는 원격 히스토리를 덮어씁니다. 계속할까요?",
			Default: false,
		}, &ok)
		if !ok {
			ui.Warn("취소되었습니다")
			return
		}
		if err := git.RunLive("push", "--force-with-lease", "-u", "origin", branch); err != nil {
			ui.Fail("강제 푸시 실패")
			return
		}
	} else {
		if err := git.RunLive("push", "-u", "origin", branch); err != nil {
			ui.Fail("푸시 실패")
			return
		}
	}

	fmt.Println()
	ui.Success("푸시 완료!")
	fmt.Println()
}
