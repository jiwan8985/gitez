package cmd

import (
	"fmt"
	"gez/internal/git"
	"gez/internal/ui"
	"os"

	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:     "status",
	Aliases: []string{"s", "st"},
	Short:   "현재 저장소 상태 표시",
	Run: func(cmd *cobra.Command, args []string) {
		if !git.IsRepo() {
			ui.Fail("git 저장소가 아닙니다")
			os.Exit(1)
		}
		runStatus()
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)
}

func runStatus() {
	branch := git.CurrentBranch()

	ahead, behind := git.AheadBehind()
	var syncParts []string
	if ahead != "" && ahead != "0" {
		syncParts = append(syncParts, ui.Green(fmt.Sprintf("%s commit 앞서 있음 (push 필요)", ahead)))
	}
	if behind != "" && behind != "0" {
		syncParts = append(syncParts, ui.Red(fmt.Sprintf("%s commit 뒤처짐 (pull 필요)", behind)))
	}

	fmt.Println()
	fmt.Printf("  %s  %s\n", ui.Bold("Branch:"), ui.BoldCyan(branch))

	for _, s := range syncParts {
		fmt.Printf("          %s\n", s)
	}
	fmt.Println()

	lines := git.StatusShort()
	if len(lines) == 0 {
		ui.Success("워킹 트리가 깨끗합니다")
		fmt.Println()
		return
	}

	// Categorise: staged / unstaged / untracked
	var staged, unstaged, untracked []string
	for _, l := range lines {
		if len(l) < 3 {
			continue
		}
		x, y := string(l[0]), string(l[1])
		file := l[3:]
		switch {
		case x == "?" && y == "?":
			untracked = append(untracked, file)
		default:
			if x != " " && x != "?" {
				staged = append(staged, fmt.Sprintf("%s  %s", ui.ColorXY(x+" "), file))
			}
			if y != " " && y != "?" {
				unstaged = append(unstaged, fmt.Sprintf("%s  %s", ui.ColorXY(" "+y), file))
			}
		}
	}

	printSection := func(title string, items []string) {
		if len(items) == 0 {
			return
		}
		fmt.Printf("  %s\n", ui.Bold(title))
		for _, item := range items {
			fmt.Printf("    %s\n", item)
		}
		fmt.Println()
	}

	printSection("스테이징됨:", staged)
	printSection("변경사항 (미스테이징):", unstaged)

	if len(untracked) > 0 {
		fmt.Printf("  %s\n", ui.Bold("추적되지 않는 파일:"))
		for _, f := range untracked {
			fmt.Printf("    %s  %s\n", ui.Blue("?"), f)
		}
		fmt.Println()
	}

	// Tip hint
	if len(staged) > 0 {
		fmt.Printf("  %s  %s\n\n", ui.Dim("Tip:"), ui.Dim("gez c  →  커밋 마법사 실행"))
	} else {
		fmt.Printf("  %s  %s\n\n", ui.Dim("Tip:"), ui.Dim("gez c  →  파일 스테이징 + 커밋"))
	}
}
