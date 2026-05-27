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

var worktreeCmd = &cobra.Command{
	Use:     "worktree",
	Aliases: []string{"wt"},
	Short:   "Git 워크트리 관리 (add·list·remove·prune)",
	Run: func(cmd *cobra.Command, args []string) {
		if !git.IsRepo() {
			ui.Fail("git 저장소가 아닙니다")
			os.Exit(1)
		}
		runWorktreeMenu()
	},
}

var worktreeListCmd = &cobra.Command{
	Use:   "list",
	Short: "워크트리 목록 보기",
	Run: func(cmd *cobra.Command, args []string) {
		printWorktreeList()
	},
}

var worktreeAddCmd = &cobra.Command{
	Use:   "add <경로> [브랜치]",
	Short: "새 워크트리 추가",
	Args:  cobra.RangeArgs(1, 2),
	Run: func(cmd *cobra.Command, args []string) {
		path := args[0]
		branch := ""
		if len(args) > 1 {
			branch = args[1]
		}
		doWorktreeAdd(path, branch)
	},
}

var worktreeRemoveCmd = &cobra.Command{
	Use:   "remove <경로>",
	Short: "워크트리 제거",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		doWorktreeRemove(args[0])
	},
}

var worktreePruneCmd = &cobra.Command{
	Use:   "prune",
	Short: "존재하지 않는 워크트리 경로 정리",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println()
		if err := git.RunLive("worktree", "prune", "-v"); err != nil {
			ui.Fail("prune 실패")
		} else {
			ui.Success("워크트리 정리 완료")
		}
		fmt.Println()
	},
}

func init() {
	worktreeCmd.AddCommand(worktreeListCmd)
	worktreeCmd.AddCommand(worktreeAddCmd)
	worktreeCmd.AddCommand(worktreeRemoveCmd)
	worktreeCmd.AddCommand(worktreePruneCmd)
	rootCmd.AddCommand(worktreeCmd)
}

// ── 대화형 메뉴 ───────────────────────────────────────────────────────────────

func runWorktreeMenu() {
	printWorktreeList()

	entries := git.WorktreeList()
	removable := make([]string, 0)
	for _, e := range entries[1:] { // skip main worktree
		removable = append(removable, e.Path)
	}

	actions := []string{"새 워크트리 추가 (add)"}
	if len(removable) > 0 {
		actions = append(actions, "워크트리 제거 (remove)")
	}
	actions = append(actions, "사용하지 않는 워크트리 정리 (prune)", "취소")

	var action string
	if err := survey.AskOne(&survey.Select{
		Message: "작업 선택:",
		Options: actions,
	}, &action); err != nil || action == "취소" {
		return
	}

	switch {
	case strings.HasPrefix(action, "새 워크트리"):
		var path string
		if err := survey.AskOne(&survey.Input{
			Message: "워크트리 경로:",
			Help:    "예: ../myrepo-hotfix",
		}, &path, survey.WithValidator(survey.Required)); err != nil {
			return
		}
		var branch string
		_ = survey.AskOne(&survey.Input{
			Message: "브랜치 이름 (없으면 새 브랜치 생성):",
			Help:    "기존 브랜치는 그대로 사용, 없으면 -b 옵션으로 새 브랜치 생성",
		}, &branch)
		doWorktreeAdd(strings.TrimSpace(path), strings.TrimSpace(branch))

	case strings.HasPrefix(action, "워크트리 제거"):
		var selected string
		if err := survey.AskOne(&survey.Select{
			Message: "제거할 워크트리:",
			Options: removable,
		}, &selected); err != nil {
			return
		}
		doWorktreeRemove(selected)

	case strings.HasPrefix(action, "사용하지"):
		fmt.Println()
		_ = git.RunLive("worktree", "prune", "-v")
		fmt.Println()
		ui.Success("정리 완료")
	}
}

// ── 실행 로직 ─────────────────────────────────────────────────────────────────

func printWorktreeList() {
	entries := git.WorktreeList()
	fmt.Println()
	fmt.Printf("  %s  (%d개)\n\n", ui.Bold("워크트리 목록"), len(entries))

	for i, e := range entries {
		label := ""
		if i == 0 {
			label = ui.Green(" (주 워크트리)")
		}
		if e.Bare {
			label = ui.Dim(" (bare)")
		}
		if e.Prunable {
			label = ui.Red(" ⚠ prunable")
		}
		branchPart := ui.Cyan(e.Branch)
		if e.Branch == "" {
			branchPart = ui.Dim("(detached) " + e.Head[:min7(e.Head)])
		}
		fmt.Printf("  %s  %s  %s%s\n",
			ui.Yellow(fmt.Sprintf("[%d]", i)),
			e.Path,
			branchPart,
			label)
	}
	fmt.Println()
}

func doWorktreeAdd(path, branch string) {
	fmt.Println()
	var args []string
	if branch == "" {
		// Ask what to do
		locals := git.LocalBranches()
		options := append([]string{"새 브랜치 생성"}, locals...)
		var sel string
		if err := survey.AskOne(&survey.Select{
			Message: "브랜치 선택 또는 새로 생성:",
			Options: options,
		}, &sel); err != nil {
			return
		}
		if sel == "새 브랜치 생성" {
			var newBranch string
			if err := survey.AskOne(&survey.Input{
				Message: "새 브랜치 이름:",
			}, &newBranch, survey.WithValidator(survey.Required)); err != nil {
				return
			}
			args = []string{"worktree", "add", "-b", strings.TrimSpace(newBranch), path}
		} else {
			args = []string{"worktree", "add", path, sel}
		}
	} else {
		// Check if branch exists
		if containsStr(git.LocalBranches(), branch) {
			args = []string{"worktree", "add", path, branch}
		} else {
			args = []string{"worktree", "add", "-b", branch, path}
		}
	}

	if err := git.RunLive(args...); err != nil {
		ui.Fail("워크트리 추가 실패")
		return
	}
	fmt.Println()
	ui.Success(fmt.Sprintf("워크트리 추가 완료: %s", path))
	fmt.Printf("  %s  cd %s  →  해당 디렉토리에서 작업하세요\n\n", ui.Dim("Tip:"), path)
}

func doWorktreeRemove(path string) {
	var ok bool
	_ = survey.AskOne(&survey.Confirm{
		Message: fmt.Sprintf("'%s' 워크트리를 제거할까요?", path),
		Default: false,
	}, &ok)
	if !ok {
		return
	}
	fmt.Println()
	if err := git.RunLive("worktree", "remove", path); err != nil {
		// Try force
		var force bool
		_ = survey.AskOne(&survey.Confirm{
			Message: "강제 제거할까요? (미커밋 변경사항 손실 주의)",
			Default: false,
		}, &force)
		if force {
			_ = git.RunLive("worktree", "remove", "--force", path)
		}
		return
	}
	ui.Success(fmt.Sprintf("'%s' 워크트리 제거 완료", path))
	fmt.Println()
}

func min7(s string) int {
	if len(s) < 7 {
		return len(s)
	}
	return 7
}
