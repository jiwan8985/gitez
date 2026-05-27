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

// ── 서브커맨드 정의 ────────────────────────────────────────────────────────

var branchCmd = &cobra.Command{
	Use:     "branch",
	Aliases: []string{"b"},
	Short:   "브랜치 관리 (전환·생성·삭제)",
	Run: func(cmd *cobra.Command, args []string) {
		if !git.IsRepo() {
			ui.Fail("git 저장소가 아닙니다")
			os.Exit(1)
		}
		runBranchMenu()
	},
}

var branchListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "브랜치 목록 보기",
	Run: func(cmd *cobra.Command, args []string) { runBranchList() },
}

var branchSwitchCmd = &cobra.Command{
	Use:     "switch [name]",
	Aliases: []string{"sw", "checkout"},
	Short:   "브랜치 전환 (인수 없으면 대화형 선택)",
	Args:    cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) > 0 {
			doSwitchTo(args[0])
		} else {
			runBranchSwitch()
		}
	},
}

var branchCreateCmd = &cobra.Command{
	Use:     "create [name]",
	Aliases: []string{"new"},
	Short:   "새 브랜치 생성",
	Args:    cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) > 0 {
			doCreateBranch(args[0])
		} else {
			runBranchCreateInteractive()
		}
	},
}

var branchDeleteCmd = &cobra.Command{
	Use:     "delete [name]",
	Aliases: []string{"del", "rm"},
	Short:   "브랜치 삭제 (인수 없으면 대화형 선택)",
	Args:    cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) > 0 {
			doDeleteBranch(args[0], false)
		} else {
			runBranchDelete()
		}
	},
}

func init() {
	branchCmd.AddCommand(branchListCmd)
	branchCmd.AddCommand(branchSwitchCmd)
	branchCmd.AddCommand(branchCreateCmd)
	branchCmd.AddCommand(branchDeleteCmd)
	rootCmd.AddCommand(branchCmd)
}

// ── 대화형 메인 메뉴 ──────────────────────────────────────────────────────

func runBranchMenu() {
	runBranchList()

	var action string
	_ = survey.AskOne(&survey.Select{
		Message: "작업 선택:",
		Options: []string{
			"브랜치 전환",
			"새 브랜치 생성",
			"브랜치 삭제",
			"취소",
		},
	}, &action)

	switch action {
	case "브랜치 전환":
		runBranchSwitch()
	case "새 브랜치 생성":
		runBranchCreateInteractive()
	case "브랜치 삭제":
		runBranchDelete()
	}
}

// ── 목록 ──────────────────────────────────────────────────────────────────

func runBranchList() {
	details := git.BranchDetails()
	remotes := git.RemoteBranches()

	fmt.Println()
	fmt.Println(ui.Bold("  로컬 브랜치:"))

	if len(details) == 0 {
		fmt.Println(ui.Dim("    브랜치 없음"))
	} else {
		// Compute name column width
		nameW := 4
		for _, d := range details {
			if len(d.Name) > nameW {
				nameW = len(d.Name)
			}
		}
		for _, d := range details {
			marker := "  "
			name := fmt.Sprintf("%-*s", nameW, d.Name)
			info := fmt.Sprintf("%s  %s  %s",
				ui.Dim(d.Date),
				ui.Yellow(d.Hash),
				ui.Dim(d.Author))
			subject := ui.Dim(truncate(d.Subject, 40))
			if d.Current {
				marker = ui.Green("* ")
				name = ui.BoldCyan(fmt.Sprintf("%-*s", nameW, d.Name))
				subject = d.Subject
			}
			fmt.Printf("    %s%s  %s  %s\n", marker, name, info, subject)
		}
	}

	if len(remotes) > 0 {
		fmt.Println()
		fmt.Println(ui.Bold("  원격 브랜치:"))
		for _, b := range remotes {
			fmt.Printf("       %s\n", ui.Dim(b))
		}
	}
	fmt.Println()
}

// ── 전환 ──────────────────────────────────────────────────────────────────

func runBranchSwitch() {
	current := git.CurrentBranch()
	locals := git.LocalBranches()

	var options []string
	for _, b := range locals {
		if b != current {
			options = append(options, b)
		}
	}

	// Also offer remote branches (strip "origin/" prefix) as checkout targets
	for _, r := range git.RemoteBranches() {
		short := strings.TrimPrefix(r, "origin/")
		if short != current && !contains(locals, short) {
			options = append(options, fmt.Sprintf("%s  %s", short, ui.Dim("(remote)")))
		}
	}

	if len(options) == 0 {
		ui.Info("전환할 수 있는 다른 브랜치가 없습니다")
		return
	}

	var selected string
	if err := survey.AskOne(&survey.Select{
		Message: "전환할 브랜치:",
		Options: options,
	}, &selected); err != nil {
		return
	}

	// Strip the dim "(remote)" annotation if present
	name := strings.Fields(selected)[0]
	doSwitchTo(name)
}

func doSwitchTo(branch string) {
	if _, err := git.Run("checkout", branch); err != nil {
		ui.Fail("브랜치 전환 실패: " + err.Error())
		return
	}
	ui.Success(fmt.Sprintf("'%s' 브랜치로 전환했습니다", branch))
	fmt.Println()
}

// ── 생성 ──────────────────────────────────────────────────────────────────

func runBranchCreateInteractive() {
	var name string
	if err := survey.AskOne(&survey.Input{
		Message: "새 브랜치 이름:",
		Help:    "예: feature/login  fix/issue-42  release/v2.0",
	}, &name, survey.WithValidator(survey.Required)); err != nil {
		return
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return
	}
	doCreateBranch(name)
}

func doCreateBranch(name string) {
	var switchNow bool
	_ = survey.AskOne(&survey.Confirm{
		Message: fmt.Sprintf("'%s' 생성 후 바로 전환할까요?", name),
		Default: true,
	}, &switchNow)

	var err error
	if switchNow {
		_, err = git.Run("checkout", "-b", name)
	} else {
		_, err = git.Run("branch", name)
	}

	if err != nil {
		ui.Fail("브랜치 생성 실패: " + err.Error())
		return
	}

	if switchNow {
		ui.Success(fmt.Sprintf("'%s' 브랜치 생성 + 전환 완료", name))
	} else {
		ui.Success(fmt.Sprintf("'%s' 브랜치 생성 완료", name))
	}
	fmt.Println()
}

// ── 삭제 ──────────────────────────────────────────────────────────────────

func runBranchDelete() {
	current := git.CurrentBranch()
	locals := git.LocalBranches()

	var options []string
	for _, b := range locals {
		if b != current {
			options = append(options, b)
		}
	}

	if len(options) == 0 {
		ui.Info("삭제 가능한 브랜치가 없습니다 (현재 브랜치는 삭제 불가)")
		return
	}

	var selected []string
	if err := survey.AskOne(&survey.MultiSelect{
		Message: "삭제할 브랜치 선택 (Space=선택, Enter=확인):",
		Options: options,
	}, &selected); err != nil || len(selected) == 0 {
		ui.Warn("취소되었습니다")
		return
	}

	// Confirm
	var ok bool
	_ = survey.AskOne(&survey.Confirm{
		Message: fmt.Sprintf("%d개 브랜치를 삭제할까요?", len(selected)),
		Default: false,
	}, &ok)
	if !ok {
		ui.Warn("취소되었습니다")
		return
	}

	for _, b := range selected {
		doDeleteBranch(b, false)
	}
}

func doDeleteBranch(name string, force bool) {
	flag := "-d"
	if force {
		flag = "-D"
	}

	if _, err := git.Run("branch", flag, name); err != nil {
		if !force && strings.Contains(err.Error(), "not fully merged") {
			var forceIt bool
			_ = survey.AskOne(&survey.Confirm{
				Message: fmt.Sprintf("'%s'은 아직 머지되지 않았습니다. 강제 삭제할까요?", name),
				Default: false,
			}, &forceIt)
			if forceIt {
				doDeleteBranch(name, true)
			}
			return
		}
		ui.Fail(fmt.Sprintf("삭제 실패 (%s): %s", name, err.Error()))
		return
	}
	ui.Success(fmt.Sprintf("'%s' 브랜치 삭제 완료", name))
}

// ── 헬퍼 ──────────────────────────────────────────────────────────────────

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
