package cmd

import (
	"fmt"
	"gez/internal/git"
	"gez/internal/ui"
	"os"
	"path/filepath"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"
)

var patchCmd = &cobra.Command{
	Use:   "patch",
	Short: "패치 파일 생성·적용 (format-patch / apply)",
	Run: func(cmd *cobra.Command, args []string) {
		if !git.IsRepo() {
			ui.Fail("git 저장소가 아닙니다")
			os.Exit(1)
		}
		runPatchMenu()
	},
}

var patchCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "커밋을 .patch 파일로 내보내기",
	Run: func(cmd *cobra.Command, args []string) {
		doPatchCreate()
	},
}

var patchApplyCmd = &cobra.Command{
	Use:   "apply <파일>",
	Short: ".patch 파일 적용",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		doPatchApply(args[0])
	},
}

func init() {
	patchCmd.AddCommand(patchCreateCmd)
	patchCmd.AddCommand(patchApplyCmd)
	rootCmd.AddCommand(patchCmd)
}

func runPatchMenu() {
	// List existing .patch files in current dir
	patches, _ := filepath.Glob("*.patch")
	fmt.Println()
	if len(patches) > 0 {
		fmt.Printf("  %s  (%d개)\n", ui.Bold("현재 디렉토리의 .patch 파일"), len(patches))
		for _, p := range patches {
			fmt.Printf("    %s  %s\n", ui.Cyan("·"), p)
		}
		fmt.Println()
	}

	var action string
	if err := survey.AskOne(&survey.Select{
		Message: "작업 선택:",
		Options: []string{
			"패치 파일 생성 (format-patch)",
			"패치 파일 적용 (apply)",
			"패치 미리보기 (apply --check)",
			"취소",
		},
	}, &action); err != nil || action == "취소" {
		return
	}

	fmt.Println()
	switch {
	case strings.HasPrefix(action, "패치 파일 생성"):
		doPatchCreate()
	case strings.HasPrefix(action, "패치 파일 적용"):
		doPatchApplyInteractive(patches)
	case strings.HasPrefix(action, "패치 미리보기"):
		doPatchCheckInteractive(patches)
	}
}

func doPatchCreate() {
	commits := git.RecentCommits(20)
	if len(commits) == 0 {
		ui.Info("커밋 기록이 없습니다")
		return
	}

	fmt.Println()
	var patchType string
	if err := survey.AskOne(&survey.Select{
		Message: "생성 범위:",
		Options: []string{
			"마지막 N개 커밋",
			"특정 커밋 하나",
			"브랜치 간 diff",
		},
	}, &patchType); err != nil {
		return
	}

	var outDir string
	if err := survey.AskOne(&survey.Input{
		Message: "저장 디렉토리:",
		Default: ".",
	}, &outDir); err != nil {
		return
	}
	outDir = strings.TrimSpace(outDir)
	if outDir == "" {
		outDir = "."
	}

	fmt.Println()
	switch {
	case strings.HasPrefix(patchType, "마지막 N"):
		nOptions := []string{"1", "2", "3", "5", "10"}
		var n string
		if err := survey.AskOne(&survey.Select{
			Message: "커밋 수:",
			Options: nOptions,
		}, &n); err != nil {
			return
		}
		if err := git.RunLive("format-patch", "-"+n, "-o", outDir); err != nil {
			ui.Fail("패치 생성 실패")
			return
		}

	case strings.HasPrefix(patchType, "특정 커밋"):
		var sel string
		if err := survey.AskOne(&survey.Select{
			Message: "커밋 선택:",
			Options: commits,
		}, &sel); err != nil {
			return
		}
		hash := strings.Fields(sel)[0]
		if err := git.RunLive("format-patch", "-1", hash, "-o", outDir); err != nil {
			ui.Fail("패치 생성 실패")
			return
		}

	case strings.HasPrefix(patchType, "브랜치 간"):
		branches := git.LocalBranches()
		var base string
		if err := survey.AskOne(&survey.Select{
			Message: "기준 브랜치 (base):",
			Options: branches,
		}, &base); err != nil {
			return
		}
		if err := git.RunLive("format-patch", base, "-o", outDir); err != nil {
			ui.Fail("패치 생성 실패")
			return
		}
	}

	fmt.Println()
	ui.Success(fmt.Sprintf("패치 파일 생성 완료! → %s/", outDir))
	fmt.Println()
}

func doPatchApply(file string) {
	fmt.Println()
	ui.Info(fmt.Sprintf("패치 적용: %s", file))

	// Check first
	if _, err := git.Run("apply", "--check", file); err != nil {
		ui.Warn("패치 충돌 가능성이 있습니다. 계속할까요?")
		var ok bool
		_ = survey.AskOne(&survey.Confirm{
			Message: "강제 적용할까요?",
			Default: false,
		}, &ok)
		if !ok {
			return
		}
	}

	var method string
	if err := survey.AskOne(&survey.Select{
		Message: "적용 방법:",
		Options: []string{
			"git apply — working tree에만 적용",
			"git am    — 커밋으로 적용 (작성자 정보 포함)",
		},
	}, &method); err != nil {
		return
	}

	fmt.Println()
	if strings.HasPrefix(method, "git apply") {
		if err := git.RunLive("apply", file); err != nil {
			ui.Fail("apply 실패")
			return
		}
	} else {
		if err := git.RunLive("am", file); err != nil {
			ui.Warn("am 실패 — 충돌 해결 후 git am --continue 실행")
			return
		}
	}
	ui.Success(fmt.Sprintf("패치 적용 완료: %s", file))
	fmt.Println()
}

func doPatchApplyInteractive(patches []string) {
	if len(patches) == 0 {
		var path string
		if err := survey.AskOne(&survey.Input{
			Message: "패치 파일 경로:",
		}, &path, survey.WithValidator(survey.Required)); err != nil {
			return
		}
		doPatchApply(strings.TrimSpace(path))
		return
	}
	var sel string
	if err := survey.AskOne(&survey.Select{
		Message: "적용할 패치 파일:",
		Options: patches,
	}, &sel); err != nil {
		return
	}
	doPatchApply(sel)
}

func doPatchCheckInteractive(patches []string) {
	if len(patches) == 0 {
		var path string
		if err := survey.AskOne(&survey.Input{
			Message: "패치 파일 경로:",
		}, &path, survey.WithValidator(survey.Required)); err != nil {
			return
		}
		patches = []string{strings.TrimSpace(path)}
	}
	var sel string
	if err := survey.AskOne(&survey.Select{
		Message: "미리볼 패치 파일:",
		Options: patches,
	}, &sel); err != nil {
		return
	}
	fmt.Println()
	if _, err := git.Run("apply", "--check", "--verbose", sel); err != nil {
		ui.Warn("충돌이 발생합니다")
	} else {
		ui.Success("충돌 없이 적용 가능합니다")
	}
	fmt.Println()
}
