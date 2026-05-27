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

var flowReleaseCmd = &cobra.Command{
	Use:   "release",
	Short: "Release 브랜치 관리 (start·finish·publish) — Git Flow 전용",
	Run: func(cmd *cobra.Command, args []string) {
		cfg := requireFlow()
		if cfg == nil {
			return
		}
		if cfg.Strategy != flow.StrategyGitFlow {
			ui.Warn("release 브랜치는 Git Flow 전략에서만 사용합니다")
			return
		}
		runFlowReleaseMenu(cfg)
	},
}

var flowReleaseStartCmd = &cobra.Command{
	Use:   "start <버전>",
	Short: "새 release 브랜치 시작 (develop 기준)",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		cfg := requireFlowGitFlow()
		if cfg == nil {
			return
		}
		doReleaseStart(cfg, args[0])
	},
}

var flowReleaseFinishCmd = &cobra.Command{
	Use:   "finish [버전]",
	Short: "Release 완료 → main + develop 머지 + 태그",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		cfg := requireFlowGitFlow()
		if cfg == nil {
			return
		}
		version := currentFlowName(cfg, "release", args)
		if version == "" {
			return
		}
		doReleaseFinish(cfg, version)
	},
}

var flowReleasePublishCmd = &cobra.Command{
	Use:   "publish [버전]",
	Short: "Release 브랜치를 원격에 push",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		cfg := requireFlowGitFlow()
		if cfg == nil {
			return
		}
		version := currentFlowName(cfg, "release", args)
		if version == "" {
			return
		}
		branch := cfg.ReleaseBranch(version)
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
	flowReleaseCmd.AddCommand(flowReleaseStartCmd)
	flowReleaseCmd.AddCommand(flowReleaseFinishCmd)
	flowReleaseCmd.AddCommand(flowReleasePublishCmd)
	flowCmd.AddCommand(flowReleaseCmd)
}

// ── 대화형 메뉴 ───────────────────────────────────────────────────────────────

func runFlowReleaseMenu(cfg *flow.Config) {
	releases := cfg.ActiveReleases()
	fmt.Println()
	if len(releases) > 0 {
		fmt.Printf("  %s  (%d개)\n", ui.Bold("활성 Release 브랜치"), len(releases))
		current := git.CurrentBranch()
		for _, r := range releases {
			branch := cfg.ReleaseBranch(r)
			star := ""
			if branch == current {
				star = ui.Green("  ◀ 현재")
			}
			fmt.Printf("    %s  %s%s\n", ui.Blue("◉"), ui.Cyan(branch), star)
		}
	} else {
		fmt.Printf("  %s\n", ui.Dim("활성 release 브랜치 없음"))
	}
	fmt.Println()

	actions := []string{"새 release 시작 (start)"}
	if len(releases) > 0 {
		actions = append(actions,
			"release 완료 (finish)  — main+develop 머지 + 태그",
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
	case strings.HasPrefix(action, "새 release"):
		var version string
		if err := survey.AskOne(&survey.Input{
			Message: "버전 (태그로 사용됩니다):",
			Help:    fmt.Sprintf("예: 1.0.0  →  브랜치 %s1.0.0, 태그 %s1.0.0", cfg.ReleasePrefix, cfg.TagPrefix),
		}, &version, survey.WithValidator(survey.Required)); err != nil {
			return
		}
		doReleaseStart(cfg, strings.TrimSpace(version))

	case strings.HasPrefix(action, "release 완료"):
		ver := selectFlowBranch(releases, "완료할 release 선택:")
		if ver != "" {
			doReleaseFinish(cfg, ver)
		}

	case strings.HasPrefix(action, "원격에 publish"):
		ver := selectFlowBranch(releases, "publish할 release 선택:")
		if ver != "" {
			branch := cfg.ReleaseBranch(ver)
			fmt.Println()
			_ = git.RunLive("push", "-u", "origin", branch)
			fmt.Println()
		}
	}
}

// ── 실행 로직 ─────────────────────────────────────────────────────────────────

func doReleaseStart(cfg *flow.Config, version string) {
	branch := cfg.ReleaseBranch(version)
	base := cfg.DevelopBranch

	fmt.Println()
	fmt.Printf("  %s  %s  %s  %s\n",
		ui.Bold("Release start:"),
		ui.Dim(base), ui.Dim("→"), ui.BoldCyan(branch))
	fmt.Println()

	// develop 최신화
	if _, err := git.Run("checkout", base); err != nil {
		ui.Fail(fmt.Sprintf("'%s' 이동 실패: %s", base, err.Error()))
		return
	}
	_ = git.RunLive("pull", "--ff-only")

	// release 브랜치 생성
	if _, err := git.Run("checkout", "-b", branch); err != nil {
		ui.Fail("브랜치 생성 실패: " + err.Error())
		return
	}

	ui.Success(fmt.Sprintf("'%s' 브랜치 생성 완료!", branch))
	fmt.Printf("  %s\n", ui.Dim("버전 번호 업데이트, CHANGELOG 작성 후 gez flow release finish 로 완료하세요"))
	fmt.Println()
}

func doReleaseFinish(cfg *flow.Config, version string) {
	branch := cfg.ReleaseBranch(version)
	tagName := cfg.TagName(version)
	main := cfg.MainBranch
	develop := cfg.DevelopBranch

	fmt.Println()
	fmt.Printf("  %s\n", ui.Bold(fmt.Sprintf("Release finish: %s", branch)))
	fmt.Printf("  %s  %s → %s (머지)\n", ui.Dim("1."), ui.Cyan(branch), ui.BoldCyan(main))
	fmt.Printf("  %s  태그 %s 생성\n", ui.Dim("2."), ui.Yellow(tagName))
	fmt.Printf("  %s  %s → %s (머지)\n", ui.Dim("3."), ui.Cyan(branch), ui.Cyan(develop))
	fmt.Printf("  %s  %s 브랜치 삭제\n", ui.Dim("4."), ui.Dim(branch))
	fmt.Println()

	// 태그 메시지
	var tagMsg string
	if err := survey.AskOne(&survey.Input{
		Message: fmt.Sprintf("태그 '%s' 메시지:", tagName),
		Default: fmt.Sprintf("Release %s", tagName),
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
	mergeMsg := fmt.Sprintf("Merge release '%s' into %s", version, main)
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

	// 3. develop 머지
	if _, err := git.Run("checkout", develop); err != nil {
		ui.Warn(fmt.Sprintf("'%s' 이동 실패 — 수동으로 머지하세요", develop))
	} else {
		_ = git.RunLive("pull", "--ff-only")
		devMergeMsg := fmt.Sprintf("Merge release '%s' into %s", version, develop)
		if err := git.RunLive("merge", "--no-ff", branch, "-m", devMergeMsg); err != nil {
			ui.Warn("develop 머지 충돌 — 수동으로 해결하세요")
		} else {
			ui.Success(fmt.Sprintf("'%s' → '%s' 머지 완료", branch, develop))
		}
	}

	// 4. release 브랜치 삭제
	_, _ = git.Run("checkout", main)
	if _, err := git.Run("branch", "-d", branch); err != nil {
		ui.Warn("로컬 브랜치 삭제 실패 (수동 삭제 필요)")
	} else {
		ui.Success(fmt.Sprintf("'%s' 브랜치 삭제 완료", branch))
	}

	fmt.Println()
	ui.Success(fmt.Sprintf("Release %s 완료! 🎉", tagName))

	// 원격 push 여부
	var pushAll bool
	if err := survey.AskOne(&survey.Confirm{
		Message: fmt.Sprintf("main, develop, 태그 '%s' 를 origin에 push할까요?", tagName),
		Default: true,
	}, &pushAll); err == nil && pushAll {
		fmt.Println()
		_ = git.RunLive("push", "origin", main)
		_ = git.RunLive("push", "origin", develop)
		_ = git.RunLive("push", "origin", tagName)
		// 원격 release 브랜치 삭제
		_ = git.RunLive("push", "origin", "--delete", branch)
		fmt.Println()
		ui.Success("원격 push 완료!")
	}
	fmt.Println()
}

// ── 가드 헬퍼 ─────────────────────────────────────────────────────────────────

func requireFlow() *flow.Config {
	cfg, err := flow.Load()
	if err != nil {
		ui.Fail("flow 설정 로드 실패: " + err.Error())
		return nil
	}
	if !cfg.IsInitialised() {
		ui.Warn("flow 전략 미설정 — gez flow init 으로 먼저 설정하세요")
		return nil
	}
	return cfg
}

func requireFlowGitFlow() *flow.Config {
	cfg := requireFlow()
	if cfg == nil {
		return nil
	}
	if cfg.Strategy != flow.StrategyGitFlow {
		ui.Warn("이 명령은 Git Flow 전략에서만 사용 가능합니다")
		return nil
	}
	return cfg
}
