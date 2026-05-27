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

var undoCmd = &cobra.Command{
	Use:   "undo",
	Short: "마지막 git 작업 취소 (reflog 기반 안전 undo)",
	Run: func(cmd *cobra.Command, args []string) {
		if !git.IsRepo() {
			ui.Fail("git 저장소가 아닙니다")
			os.Exit(1)
		}
		runUndo()
	},
}

func init() {
	rootCmd.AddCommand(undoCmd)
}

func runUndo() {
	// Get recent reflog to detect last operation
	rawEntries := git.ReflogEntries(20)
	if len(rawEntries) == 0 {
		ui.Info("reflog 기록이 없습니다")
		return
	}
	entries := rawEntries

	fmt.Println()
	fmt.Println(ui.Bold("  최근 작업 기록 (reflog):"))
	for i, e := range entries {
		prefix := "  "
		if i == 0 {
			prefix = ui.Green("▶ ")
		}
		fmt.Printf("    %s%s\n", prefix, ui.Dim(e))
	}
	fmt.Println()

	// Detect last operation type from reflog
	lastEntry := entries[0]
	opType := detectLastOperation(lastEntry)

	fmt.Printf("  %s  %s\n\n", ui.Bold("마지막 작업:"), ui.BoldYellow(opType))

	var action string
	options := buildUndoOptions(opType)
	if err := survey.AskOne(&survey.Select{
		Message: "취소 방법 선택:",
		Options: options,
	}, &action); err != nil || action == "취소" {
		return
	}

	fmt.Println()
	executeUndo(action, entries)
}

func detectLastOperation(entry string) string {
	lower := strings.ToLower(entry)
	switch {
	case strings.Contains(lower, "commit"):
		return "커밋"
	case strings.Contains(lower, "merge"):
		return "머지"
	case strings.Contains(lower, "rebase"):
		return "리베이스"
	case strings.Contains(lower, "reset"):
		return "리셋"
	case strings.Contains(lower, "checkout") || strings.Contains(lower, "switch"):
		return "브랜치 전환"
	case strings.Contains(lower, "pull"):
		return "풀"
	case strings.Contains(lower, "cherry-pick"):
		return "cherry-pick"
	case strings.Contains(lower, "revert"):
		return "리버트"
	case strings.Contains(lower, "stash"):
		return "스태시"
	default:
		return "기타"
	}
}

func buildUndoOptions(opType string) []string {
	var opts []string
	switch opType {
	case "커밋":
		opts = append(opts,
			"마지막 커밋 취소 — soft reset (변경사항 유지, staged 상태)",
			"마지막 커밋 취소 — mixed reset (변경사항 유지, unstaged)",
			"마지막 커밋 취소 — hard reset (변경사항 삭제) ⚠",
		)
	case "머지":
		opts = append(opts,
			"머지 취소 — MERGE_HEAD 이전으로 reset",
		)
	case "리베이스":
		opts = append(opts,
			"리베이스 취소 — ORIG_HEAD 이전으로 reset",
		)
	case "풀":
		opts = append(opts,
			"pull 취소 — ORIG_HEAD 이전으로 reset",
		)
	}
	opts = append(opts,
		"reflog에서 직접 선택해서 복구",
		"취소",
	)
	return opts
}

func executeUndo(action string, entries []string) {
	switch {
	case strings.Contains(action, "soft reset"):
		if _, err := git.Run("reset", "--soft", "HEAD~1"); err != nil {
			ui.Fail("reset 실패: " + err.Error())
			return
		}
		ui.Success("커밋 취소 완료 (변경사항은 staged 상태로 유지)")

	case strings.Contains(action, "mixed reset") && strings.Contains(action, "마지막 커밋"):
		if _, err := git.Run("reset", "HEAD~1"); err != nil {
			ui.Fail("reset 실패: " + err.Error())
			return
		}
		ui.Success("커밋 취소 완료 (변경사항은 unstaged 상태로 유지)")

	case strings.Contains(action, "hard reset") && strings.Contains(action, "마지막 커밋"):
		var ok bool
		_ = survey.AskOne(&survey.Confirm{
			Message: "⚠ 변경사항이 모두 삭제됩니다. 계속할까요?",
			Default: false,
		}, &ok)
		if !ok {
			ui.Warn("취소됨")
			return
		}
		if _, err := git.Run("reset", "--hard", "HEAD~1"); err != nil {
			ui.Fail("reset 실패: " + err.Error())
			return
		}
		ui.Success("커밋 취소 완료 (변경사항 삭제됨)")

	case strings.Contains(action, "MERGE_HEAD") || strings.Contains(action, "머지 취소"):
		if _, err := git.Run("reset", "--hard", "ORIG_HEAD"); err != nil {
			// Try MERGE_HEAD based approach
			if _, err2 := git.Run("merge", "--abort"); err2 != nil {
				ui.Fail("머지 취소 실패")
				return
			}
		}
		ui.Success("머지 취소 완료")

	case strings.Contains(action, "ORIG_HEAD"):
		if _, err := git.Run("reset", "--hard", "ORIG_HEAD"); err != nil {
			ui.Fail("ORIG_HEAD reset 실패: " + err.Error())
			return
		}
		ui.Success("작업 취소 완료 (ORIG_HEAD로 복구)")

	case strings.HasPrefix(action, "reflog에서 직접"):
		runReflogRecover(entries)
		return
	}

	hash, _ := git.Run("rev-parse", "--short", "HEAD")
	branch := git.CurrentBranch()
	fmt.Printf("  %s  현재: [%s] %s\n\n", ui.Dim("→"), ui.Yellow(hash), ui.Cyan(branch))
}

func runReflogRecover(entries []string) {
	options := entries

	var sel string
	if err := survey.AskOne(&survey.Select{
		Message: "복구할 시점 선택:",
		Options: options,
	}, &sel); err != nil {
		return
	}

	hash := strings.Fields(sel)[0]

	var resetMode string
	if err := survey.AskOne(&survey.Select{
		Message: "reset 모드:",
		Options: []string{
			"soft  — 커밋 취소, 변경사항 staged 유지",
			"mixed — 커밋 취소, 변경사항 unstaged 유지",
			"hard  — 커밋 취소, 변경사항 삭제 ⚠",
		},
	}, &resetMode); err != nil {
		return
	}

	mode := "--soft"
	if strings.HasPrefix(resetMode, "mixed") {
		mode = "--mixed"
	} else if strings.HasPrefix(resetMode, "hard") {
		var ok bool
		_ = survey.AskOne(&survey.Confirm{
			Message: "⚠ 변경사항이 삭제됩니다. 계속할까요?",
			Default: false,
		}, &ok)
		if !ok {
			return
		}
		mode = "--hard"
	}

	if _, err := git.Run("reset", mode, hash); err != nil {
		ui.Fail("reset 실패: " + err.Error())
		return
	}
	ui.Success(fmt.Sprintf("'%s' 시점으로 복구 완료", hash))
	fmt.Println()
}
