package cmd

import (
	"fmt"
	"gez/internal/flow"
	"gez/internal/git"
	"gez/internal/ui"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"
)

var flowHotfixCmd = &cobra.Command{
	Use:   "hotfix",
	Short: "Hotfix 브랜치 관리 (start·finish) — Git Flow 전용",
	Run: func(cmd *cobra.Command, args []string) {
		cfg := requireFlowGitFlow()
		if cfg == nil {
			return
		}
		runFlowHotfixMenu(cfg)
	},
}

var flowHotfixStartCmd = &cobra.Command{
	Use:   "start <이름>",
	Short: "새 hotfix 브랜치 시작 (main 기준)",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		cfg := requireFlowGitFlow()
		if cfg == nil {
			return
		}
		doHotfixStart(cfg, args[0])
	},
}

var flowHotfixFinishCmd = &cobra.Command{
	Use:   "finish [이름]",
	Short: "Hotfix 완료 → main + develop 머지 + 태그",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		cfg := requireFlowGitFlow()
		if cfg == nil {
			return
		}
		name := currentFlowName(cfg, "hotfix", args)
		if name == "" {
			return
		}
		doHotfixFinish(cfg, name)
	},
}

var flowHotfixPublishCmd = &cobra.Command{
	Use:   "publish [이름]",
	Short: "Hotfix 브랜치를 원격에 push",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		cfg := requireFlowGitFlow()
		if cfg == nil {
			return
		}
		name := currentFlowName(cfg, "hotfix", args)
		if name == "" {
			return
		}
		branch := cfg.HotfixBranch(name)
		fmt.Println()
		if err := git.RunLive("push", "-u", "origin", branch); err != nil {
			ui.Fail("push 실패")
			return
		}
		fmt.Println()
		ui.Success(fmt.Sprintf("'%s' 원격 push 완료!", branch))
		fmt.Println()
	},
}

func init() {
	flowHotfixCmd.AddCommand(flowHotfixStartCmd)
	flowHotfixCmd.AddCommand(flowHotfixFinishCmd)
	flowHotfixCmd.AddCommand(flowHotfixPublishCmd)
	flowCmd.AddCommand(flowHotfixCmd)
}

// ── 대화형 메뉴 ───────────────────────────────────────────────────────────────

func runFlowHotfixMenu(cfg *flow.Config) {
	hotfixes := cfg.ActiveHotfixes()
	fmt.Println()
	if len(hotfixes) > 0 {
		fmt.Printf("  %s  (%d개)\n", ui.Bold("활성 Hotfix 브랜치"), len(hotfixes))
		current := git.CurrentBranch()
		for _, h := range hotfixes {
			branch := cfg.HotfixBranch(h)
			star := ""
			if branch == current {
				star = ui.Green("  ◀ 현재")
			}
			fmt.Printf("    %s  %s%s\n", ui.Red("◉"), ui.Cyan(branch), star)
		}
	} else {
		fmt.Printf("  %s\n", ui.Dim("활성 hotfix 브랜치 없음"))
	}
	fmt.Println()

	actions := []string{"새 hotfix 시작 (start)  — 프로덕션 긴급 버그 수정"}
	if len(hotfixes) > 0 {
		actions = append(actions,
			"hotfix 완료 (finish)  — main+develop 머지 + 태그",
			"원격에 publish")
	}
	actions = append(actions, "취소")

	var action string
	if err := survey.AskOne(&survey.Select{
		Message: "작업 선택:",
		Options: actions,
	}, &action); err != nil || action == "취소" {
		return
	}

	switch {
	case strings.HasPrefix(action, "새 hotfix"):
		var name string
		if err := survey.AskOne(&survey.Input{
			Message: "Hotfix 이름 또는 버전:",
			Help:    "예: fix-login-crash  1.0.1  critical-bug",
		}, &name, survey.WithValidator(survey.Required)); err != nil {
			return
		}
		doHotfixStart(cfg, strings.TrimSpace(name))

	case strings.HasPrefix(action, "hotfix 완료"):
		name := selectFlowBranch(hotfixes, "완료할 hotfix 선택:")
		if name != "" {
			doHotfixFinish(cfg, name)
		}

	case strings.HasPrefix(action, "원격에 publish"):
		name := selectFlowBranch(hotfixes, "publish할 hotfix 선택:")
		if name != "" {
			branch := cfg.HotfixBranch(name)
			fmt.Println()
			_ = git.RunLive("push", "-u", "origin", branch)
			fmt.Println()
		}
	}
}

// ── 실행 로직 ─────────────────────────────────────────────────────────────────

func doHotfixStart(cfg *flow.Config, name string) {
	branch := cfg.HotfixBranch(name)
	base := cfg.MainBranch // hotfix는 항상 main 기준

	fmt.Println()
	fmt.Printf("  %s  %s  %s  %s\n",
		ui.Bold("Hotfix start:"),
		ui.Dim(base), ui.Dim("→"), ui.BoldCyan(branch))
	fmt.Printf("  %s  프로덕션(%s) 기준으로 생성합니다\n\n", ui.Dim("⚠"), ui.Cyan(base))

	// main 최신화
	if _, err := git.Run("checkout", base); err != nil {
		ui.Fail(fmt.Sprintf("'%s' 이동 실패: %s", base, err.Error()))
		return
	}
	_ = git.RunLive("pull", "--ff-only")

	// hotfix 브랜치 생성
	if _, err := git.Run("checkout", "-b", branch); err != nil {
		ui.Fail("브랜치 생성 실패: " + err.Error())
		return
	}

	ui.Success(fmt.Sprintf("'%s' 브랜치 생성 완료!", branch))
	fmt.Printf("  %s  버그 수정 후 %s 로 완료하세요\n\n",
		ui.Dim("Tip:"), ui.Cyan("gez flow hotfix finish"))
}

func doHotfixFinish(cfg *flow.Config, name string) {
	branch := cfg.HotfixBranch(name)
	tagName := cfg.TagName(name)
	main := cfg.MainBranch
	develop := cfg.DevelopBranch

	fmt.Println()
	fmt.Printf("  %s\n", ui.Bold(fmt.Sprintf("Hotfix finish: %s", branch)))
	fmt.Printf("  %s  %s → %s (머지)\n", ui.Dim("1."), ui.Cyan(branch), ui.BoldCyan(main))
	fmt.Printf("  %s  태그 %s 생성\n", ui.Dim("2."), ui.Yellow(tagName))
	fmt.Printf("  %s  %s → %s (머지)\n", ui.Dim("3."), ui.Cyan(branch), ui.Cyan(develop))
	fmt.Printf("  %s  %s 브랜치 삭제\n\n", ui.Dim("4."), ui.Dim(branch))

	// 태그 메시지
	var tagMsg string
	if err := survey.AskOne(&survey.Input{
		Message: fmt.Sprintf("태그 '%s' 메시지:", tagName),
		Default: fmt.Sprintf("Hotfix %s", tagName),
	}, &tagMsg); err != nil {
		return
	}

	var ok bool
	if err := survey.AskOne(&survey.Confirm{
		Message: "위 절차를 실행할까요?",
		Default: true,
	}, &ok); err != nil || !ok {
		ui.Warn("취소되었습니다")
		return
	}

	fmt.Println()

	// 1. main 머지
	if _, err := git.Run("checkout", main); err != nil {
		ui.Fail(fmt.Sprintf("'%s' 이동 실패: %s", main, err.Error()))
		return
	}
	_ = git.RunLive("pull", "--ff-only")
	mergeMsg := fmt.Sprintf("Merge hotfix '%s' into %s", name, main)
	if err := git.RunLive("merge", "--no-ff", branch, "-m", mergeMsg); err != nil {
		ui.Warn("main 머지 충돌 — 충돌 해결 후 수동으로 완료하세요")
		return
	}
	ui.Success(fmt.Sprintf("'%s' → '%s' 머지 완료", branch, main))

	// 2. 태그 생성
	if _, err := git.Run("tag", "-a", tagName, "-m", tagMsg); err != nil {
		ui.Warn("태그 생성 실패: " + err.Error())
	} else {
		ui.Success(fmt.Sprintf("태그 '%s' 생성 완료", tagName))
	}

	// 3. develop 머지 (release 진행 중이면 release에도 머지 필요하지만 일단 develop으로)
	releases := cfg.ActiveReleases()
	mergeTargets := []string{develop}
	if len(releases) > 0 {
		ui.Info(fmt.Sprintf("활성 release 브랜치 발견: %v — develop 대신 release에도 머지를 고려하세요", releases))
	}

	for _, target := range mergeTargets {
		if _, err := git.Run("checkout", target); err != nil {
			ui.Warn(fmt.Sprintf("'%s' 이동 실패 — 수동 머지 필요", target))
			continue
		}
		_ = git.RunLive("pull", "--ff-only")
		devMsg := fmt.Sprintf("Merge hotfix '%s' into %s", name, target)
		if err := git.RunLive("merge", "--no-ff", branch, "-m", devMsg); err != nil {
			ui.Warn(fmt.Sprintf("'%s' 머지 충돌 — 수동으로 해결하세요", target))
		} else {
			ui.Success(fmt.Sprintf("'%s' → '%s' 머지 완료", branch, target))
		}
	}

	// 4. hotfix 브랜치 삭제
	_, _ = git.Run("checkout", main)
	if _, err := git.Run("branch", "-d", branch); err != nil {
		ui.Warn("로컬 브랜치 삭제 실패 (수동 삭제 필요)")
	} else {
		ui.Success(fmt.Sprintf("'%s' 브랜치 삭제 완료", branch))
	}

	fmt.Println()
	ui.Success(fmt.Sprintf("Hotfix %s 완료! 🔥 버그가 수정됐습니다.", tagName))

	// 원격 push
	var pushAll bool
	if err := survey.AskOne(&survey.Confirm{
		Message: fmt.Sprintf("main, develop, 태그 '%s' 를 origin에 push할까요?", tagName),
		Default: true,
	}, &pushAll); err == nil && pushAll {
		fmt.Println()
		_ = git.RunLive("push", "origin", main)
		_ = git.RunLive("push", "origin", develop)
		_ = git.RunLive("push", "origin", tagName)
		_ = git.RunLive("push", "origin", "--delete", branch)
		fmt.Println()
		ui.Success("원격 push 완료!")
	}
	fmt.Println()
}
