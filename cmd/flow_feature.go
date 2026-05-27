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

var flowFeatureCmd = &cobra.Command{
	Use:   "feature",
	Short: "Feature 브랜치 관리 (start·finish·publish·list)",
	Run: func(cmd *cobra.Command, args []string) {
		cfg := requireFlow()
		if cfg == nil {
			return
		}
		runFlowFeatureMenu(cfg)
	},
}

// ── feature start ─────────────────────────────────────────────────────────────

var flowFeatureStartCmd = &cobra.Command{
	Use:   "start <이름>",
	Short: "새 feature 브랜치 시작",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		cfg := requireFlow()
		if cfg == nil {
			return
		}
		doFeatureStart(cfg, args[0])
	},
}

// ── feature finish ────────────────────────────────────────────────────────────

var flowFeatureFinishCmd = &cobra.Command{
	Use:   "finish [이름]",
	Short: "Feature 완료 → 통합 브랜치에 머지",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		cfg := requireFlow()
		if cfg == nil {
			return
		}
		name := currentFlowName(cfg, "feature", args)
		if name == "" {
			return
		}
		doFeatureFinish(cfg, name)
	},
}

// ── feature publish ───────────────────────────────────────────────────────────

var flowFeaturePublishCmd = &cobra.Command{
	Use:   "publish [이름]",
	Short: "Feature 브랜치를 원격에 push",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		cfg := requireFlow()
		if cfg == nil {
			return
		}
		name := currentFlowName(cfg, "feature", args)
		if name == "" {
			return
		}
		branch := cfg.FeatureBranch(name)
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

// ── feature list ──────────────────────────────────────────────────────────────

var flowFeatureListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "활성 feature 브랜치 목록",
	Run: func(cmd *cobra.Command, args []string) {
		cfg := requireFlow()
		if cfg == nil {
			return
		}
		printFeatureList(cfg)
	},
}

func init() {
	flowFeatureCmd.AddCommand(flowFeatureStartCmd)
	flowFeatureCmd.AddCommand(flowFeatureFinishCmd)
	flowFeatureCmd.AddCommand(flowFeaturePublishCmd)
	flowFeatureCmd.AddCommand(flowFeatureListCmd)
	flowCmd.AddCommand(flowFeatureCmd)
}

// ── 대화형 메뉴 ───────────────────────────────────────────────────────────────

func runFlowFeatureMenu(cfg *flow.Config) {
	printFeatureList(cfg)

	actions := []string{"새 feature 시작 (start)"}
	if len(cfg.ActiveFeatures()) > 0 {
		actions = append(actions,
			"feature 완료 (finish)",
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
	case strings.HasPrefix(action, "새 feature"):
		var name string
		if err := survey.AskOne(&survey.Input{
			Message: "Feature 이름:",
			Help:    "예: login  user-auth  issue-42",
		}, &name, survey.WithValidator(survey.Required)); err != nil {
			return
		}
		doFeatureStart(cfg, strings.TrimSpace(name))

	case strings.HasPrefix(action, "feature 완료"):
		name := selectFlowBranch(cfg.ActiveFeatures(), "완료할 feature 선택:")
		if name != "" {
			doFeatureFinish(cfg, name)
		}

	case strings.HasPrefix(action, "원격에 publish"):
		name := selectFlowBranch(cfg.ActiveFeatures(), "publish할 feature 선택:")
		if name != "" {
			branch := cfg.FeatureBranch(name)
			fmt.Println()
			if err := git.RunLive("push", "-u", "origin", branch); err != nil {
				ui.Fail("push 실패")
			} else {
				fmt.Println()
				ui.Success(fmt.Sprintf("'%s' 원격 push 완료!", branch))
			}
			fmt.Println()
		}
	}
}

// ── 실행 로직 ─────────────────────────────────────────────────────────────────

func doFeatureStart(cfg *flow.Config, name string) {
	branch := cfg.FeatureBranch(name)

	// 분기 기준: gitflow=develop, 그 외=main
	base := cfg.MainBranch
	if cfg.Strategy == flow.StrategyGitFlow {
		base = cfg.DevelopBranch
	}

	fmt.Println()
	fmt.Printf("  %s  %s  %s  %s\n",
		ui.Bold("Feature start:"),
		ui.Dim(base),
		ui.Dim("→"),
		ui.BoldCyan(branch))
	fmt.Println()

	// base 브랜치로 이동 후 최신화
	if _, err := git.Run("checkout", base); err != nil {
		ui.Fail(fmt.Sprintf("'%s' 브랜치 이동 실패: %s", base, err.Error()))
		return
	}
	_ = git.RunLive("pull", "--ff-only")

	// feature 브랜치 생성
	if _, err := git.Run("checkout", "-b", branch); err != nil {
		ui.Fail("브랜치 생성 실패: " + err.Error())
		return
	}

	ui.Success(fmt.Sprintf("'%s' 브랜치 생성 완료!", branch))
	fmt.Printf("  %s  이제 작업을 시작하고 완료 후 %s 를 실행하세요\n\n",
		ui.Dim("Tip:"), ui.Cyan("gez flow feature finish"))
}

func doFeatureFinish(cfg *flow.Config, name string) {
	branch := cfg.FeatureBranch(name)

	// 머지 대상: gitflow=develop, 그 외=main
	target := cfg.MainBranch
	if cfg.Strategy == flow.StrategyGitFlow {
		target = cfg.DevelopBranch
	}

	fmt.Println()
	fmt.Printf("  %s  %s  %s  %s\n",
		ui.Bold("Feature finish:"),
		ui.BoldCyan(branch),
		ui.Dim("→"),
		ui.Cyan(target))

	// 머지될 커밋 미리보기
	commits := git.CommitsBetween(target, branch)
	if len(commits) > 0 {
		fmt.Printf("\n  %s  머지될 커밋 (%d개):\n", ui.Dim("Preview:"), len(commits))
		for _, c := range commits {
			fmt.Printf("    %s  %s\n", ui.Yellow("·"), c)
		}
	}
	fmt.Println()

	var ok bool
	if err := survey.AskOne(&survey.Confirm{
		Message: fmt.Sprintf("'%s' 를 '%s' 에 머지하고 브랜치를 삭제할까요?", branch, target),
		Default: true,
	}, &ok); err != nil || !ok {
		ui.Warn("취소되었습니다")
		return
	}

	// target으로 이동 + 최신화
	if _, err := git.Run("checkout", target); err != nil {
		ui.Fail(fmt.Sprintf("'%s' 이동 실패: %s", target, err.Error()))
		return
	}
	_ = git.RunLive("pull", "--ff-only")

	// --no-ff 머지 (머지 커밋 생성으로 히스토리 보존)
	mergeMsg := fmt.Sprintf("Merge feature '%s' into %s", name, target)
	if err := git.RunLive("merge", "--no-ff", branch, "-m", mergeMsg); err != nil {
		fmt.Println()
		ui.Warn("머지 충돌 발생 — 충돌 해결 후 gez merge 로 계속하세요")
		return
	}

	// feature 브랜치 삭제
	if _, err := git.Run("branch", "-d", branch); err != nil {
		ui.Warn(fmt.Sprintf("로컬 브랜치 삭제 실패: %s", err.Error()))
	}

	fmt.Println()
	ui.Success(fmt.Sprintf("Feature '%s' 완료! '%s' 에 머지됐습니다.", name, target))

	// 원격 브랜치 삭제 여부
	remotes := git.RemoteBranches()
	remoteRef := "origin/" + branch
	if containsStr(remotes, remoteRef) {
		var delRemote bool
		_ = survey.AskOne(&survey.Confirm{
			Message: "원격(origin) feature 브랜치도 삭제할까요?",
			Default: true,
		}, &delRemote)
		if delRemote {
			if err := git.RunLive("push", "origin", "--delete", branch); err != nil {
				ui.Warn("원격 브랜치 삭제 실패")
			} else {
				ui.Success("원격 브랜치 삭제 완료")
			}
		}
	}
	fmt.Println()
}

// ── 헬퍼 ─────────────────────────────────────────────────────────────────────

func printFeatureList(cfg *flow.Config) {
	features := cfg.ActiveFeatures()
	fmt.Println()
	if len(features) == 0 {
		fmt.Printf("  %s\n\n", ui.Dim("활성 feature 브랜치 없음"))
		return
	}
	current := git.CurrentBranch()
	fmt.Printf("  %s  (%d개)\n", ui.Bold("활성 Feature 브랜치"), len(features))
	for _, f := range features {
		branch := cfg.FeatureBranch(f)
		star := ""
		if branch == current {
			star = ui.Green("  ◀ 현재")
		}
		fmt.Printf("    %s  %s%s\n", ui.Yellow("◉"), ui.Cyan(branch), star)
	}
	fmt.Println()
}

func selectFlowBranch(names []string, prompt string) string {
	if len(names) == 0 {
		ui.Info("선택 가능한 브랜치가 없습니다")
		return ""
	}
	if len(names) == 1 {
		return names[0]
	}
	var selected string
	if err := survey.AskOne(&survey.Select{
		Message: prompt,
		Options: names,
	}, &selected); err != nil {
		return ""
	}
	return selected
}

func currentFlowName(cfg *flow.Config, branchType string, args []string) string {
	if len(args) > 0 {
		return args[0]
	}
	// 현재 브랜치에서 이름 추출
	bType, shortName := cfg.CurrentFlowBranch()
	if bType == branchType {
		return shortName
	}
	// 목록에서 선택
	var names []string
	switch branchType {
	case "feature":
		names = cfg.ActiveFeatures()
	case "release":
		names = cfg.ActiveReleases()
	case "hotfix":
		names = cfg.ActiveHotfixes()
	}
	return selectFlowBranch(names, fmt.Sprintf("%s 선택:", branchType))
}
