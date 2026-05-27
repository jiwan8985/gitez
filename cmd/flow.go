package cmd

import (
	"fmt"
	"gez/internal/flow"
	"gez/internal/git"
	"gez/internal/ui"
	"os"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"
)

// ── Root: gez flow ────────────────────────────────────────────────────────────

var flowCmd = &cobra.Command{
	Use:   "flow",
	Short: "Git 브랜치 전략 관리 (Git Flow / GitHub Flow / Trunk)",
	Run: func(cmd *cobra.Command, args []string) {
		if !git.IsRepo() {
			ui.Fail("git 저장소가 아닙니다")
			os.Exit(1)
		}
		cfg, err := flow.Load()
		if err != nil {
			ui.Fail("flow 설정 로드 실패: " + err.Error())
			os.Exit(1)
		}
		if !cfg.IsInitialised() {
			ui.Warn("flow 전략이 설정되지 않았습니다")
			fmt.Printf("  %s\n\n", ui.Cyan("gez flow init  →  전략 선택 후 설정"))
			return
		}
		runFlowStatus(cfg)
	},
}

// ── flow init ─────────────────────────────────────────────────────────────────

var flowInitCmd = &cobra.Command{
	Use:   "init",
	Short: "브랜치 전략 초기화 (Git Flow / GitHub Flow / Trunk)",
	Run: func(cmd *cobra.Command, args []string) {
		if !git.IsRepo() {
			ui.Fail("git 저장소가 아닙니다")
			os.Exit(1)
		}
		runFlowInit()
	},
}

func runFlowInit() {
	fmt.Println()
	fmt.Printf("  %s\n\n", ui.Bold("브랜치 전략 초기화"))

	// 전략 선택
	var stratLabel string
	if err := survey.AskOne(&survey.Select{
		Message: "브랜치 전략 선택:",
		Options: []string{
			"Git Flow       — main + develop + feature/* + release/* + hotfix/*",
			"GitHub Flow    — main + feature/* (단순, PR 중심)",
			"Trunk-based    — main 하나 + 단기 feature/* (CI/CD 중심)",
		},
		Help: "팀 규모와 릴리즈 주기에 맞게 선택하세요",
	}, &stratLabel); err != nil {
		return
	}

	var strat flow.Strategy
	switch {
	case strings.HasPrefix(stratLabel, "Git Flow"):
		strat = flow.StrategyGitFlow
	case strings.HasPrefix(stratLabel, "GitHub Flow"):
		strat = flow.StrategyGitHubFlow
	default:
		strat = flow.StrategyTrunk
	}

	cfg := &flow.Config{Strategy: strat}

	fmt.Println()
	fmt.Printf("  %s\n\n", ui.Bold("브랜치 이름 설정 (Enter = 기본값 사용)"))

	// Main 브랜치
	mainBranch := "main"
	if err := survey.AskOne(&survey.Input{
		Message: "프로덕션 브랜치 이름:",
		Default: "main",
	}, &mainBranch); err != nil {
		return
	}
	cfg.MainBranch = strings.TrimSpace(mainBranch)

	if strat == flow.StrategyGitFlow {
		// Develop 브랜치
		developBranch := "develop"
		if err := survey.AskOne(&survey.Input{
			Message: "통합(develop) 브랜치 이름:",
			Default: "develop",
		}, &developBranch); err != nil {
			return
		}
		cfg.DevelopBranch = strings.TrimSpace(developBranch)
	}

	// Prefix 설정 (Git Flow / GitHub Flow)
	if strat != flow.StrategyTrunk {
		fmt.Println()
		fmt.Printf("  %s\n\n", ui.Dim("브랜치 Prefix (Enter = 기본값)"))

		featureP := "feature/"
		_ = survey.AskOne(&survey.Input{Message: "Feature prefix:", Default: "feature/"}, &featureP)
		cfg.FeaturePrefix = strings.TrimSpace(featureP)

		if strat == flow.StrategyGitFlow {
			releaseP := "release/"
			_ = survey.AskOne(&survey.Input{Message: "Release prefix:", Default: "release/"}, &releaseP)
			cfg.ReleasePrefix = strings.TrimSpace(releaseP)

			hotfixP := "hotfix/"
			_ = survey.AskOne(&survey.Input{Message: "Hotfix prefix:", Default: "hotfix/"}, &hotfixP)
			cfg.HotfixPrefix = strings.TrimSpace(hotfixP)

			tagP := "v"
			_ = survey.AskOne(&survey.Input{Message: "Version tag prefix:", Default: "v"}, &tagP)
			cfg.TagPrefix = strings.TrimSpace(tagP)
		}
	}

	// 기본값 채우기
	if cfg.FeaturePrefix == "" {
		cfg.FeaturePrefix = "feature/"
	}
	if cfg.ReleasePrefix == "" {
		cfg.ReleasePrefix = "release/"
	}
	if cfg.HotfixPrefix == "" {
		cfg.HotfixPrefix = "hotfix/"
	}
	if cfg.SupportPrefix == "" {
		cfg.SupportPrefix = "support/"
	}
	if cfg.TagPrefix == "" {
		cfg.TagPrefix = "v"
	}

	// 저장
	if err := cfg.Save(); err != nil {
		ui.Fail("설정 저장 실패: " + err.Error())
		return
	}

	// Git Flow: develop 브랜치 생성
	if strat == flow.StrategyGitFlow {
		fmt.Println()
		locals := git.LocalBranches()
		if !containsStr(locals, cfg.DevelopBranch) {
			var createDev bool
			if err := survey.AskOne(&survey.Confirm{
				Message: fmt.Sprintf("'%s' 브랜치가 없습니다. 지금 생성할까요?", cfg.DevelopBranch),
				Default: true,
			}, &createDev); err == nil && createDev {
				if _, err := git.Run("checkout", "-b", cfg.DevelopBranch); err != nil {
					ui.Warn("develop 브랜치 생성 실패: " + err.Error())
				} else {
					ui.Success(fmt.Sprintf("'%s' 브랜치 생성 완료", cfg.DevelopBranch))
					// main으로 다시
					_, _ = git.Run("checkout", cfg.MainBranch)
				}
			}
		}
	}

	fmt.Println()
	ui.Success(fmt.Sprintf("%s 전략 설정 완료!", strat.Label()))
	fmt.Println()
	runFlowStatus(cfg)
}

// ── flow status ───────────────────────────────────────────────────────────────

var flowStatusCmd = &cobra.Command{
	Use:     "status",
	Aliases: []string{"st"},
	Short:   "현재 flow 상태 보기",
	Run: func(cmd *cobra.Command, args []string) {
		if !git.IsRepo() {
			ui.Fail("git 저장소가 아닙니다")
			os.Exit(1)
		}
		cfg, _ := flow.Load()
		if !cfg.IsInitialised() {
			ui.Warn("flow 미설정 — gez flow init 으로 시작하세요")
			return
		}
		runFlowStatus(cfg)
	},
}

func runFlowStatus(cfg *flow.Config) {
	sep := ui.Dim(strings.Repeat("─", 52))
	current := git.CurrentBranch()
	branchType, shortName := cfg.CurrentFlowBranch()

	fmt.Println()
	fmt.Printf("  %s  %s\n", ui.Bold("전략:"), ui.BoldCyan(cfg.Strategy.Label()))
	fmt.Println()

	// 현재 브랜치
	branchIcon := branchIcon(branchType)
	fmt.Printf("  %s  현재 브랜치  %s  %s\n",
		branchIcon,
		ui.BoldCyan(current),
		ui.Dim(fmt.Sprintf("(%s: %s)", branchType, shortName)))
	fmt.Println()
	fmt.Println(sep)

	// 브랜치 구조 시각화
	printFlowTree(cfg)

	// 액티브 브랜치 목록
	fmt.Println(sep)
	features := cfg.ActiveFeatures()
	releases := cfg.ActiveReleases()
	hotfixes := cfg.ActiveHotfixes()

	hasActive := len(features)+len(releases)+len(hotfixes) > 0
	if hasActive {
		fmt.Printf("  %s\n", ui.Bold("진행 중인 브랜치:"))
		for _, f := range features {
			star := ""
			if branchType == "feature" && shortName == f {
				star = ui.Green(" ◀ 현재")
			}
			fmt.Printf("    %s  %s%s\n", ui.Yellow("feature"), ui.Cyan(cfg.FeatureBranch(f)), star)
		}
		for _, r := range releases {
			star := ""
			if branchType == "release" && shortName == r {
				star = ui.Green(" ◀ 현재")
			}
			fmt.Printf("    %s  %s%s\n", ui.Blue("release"), ui.Cyan(cfg.ReleaseBranch(r)), star)
		}
		for _, h := range hotfixes {
			star := ""
			if branchType == "hotfix" && shortName == h {
				star = ui.Green(" ◀ 현재")
			}
			fmt.Printf("    %s  %s%s\n", ui.Red("hotfix "), ui.Cyan(cfg.HotfixBranch(h)), star)
		}
	} else {
		fmt.Printf("  %s\n", ui.Dim("진행 중인 feature/release/hotfix 없음"))
	}

	fmt.Println(sep)
	fmt.Println()

	// 컨텍스트별 다음 명령 힌트
	printFlowHints(cfg, branchType)
}

func printFlowTree(cfg *flow.Config) {
	current := git.CurrentBranch()
	mark := func(branch string) string {
		if branch == current {
			return ui.Green("* ")
		}
		return "  "
	}
	exists := func(b string) bool {
		return containsStr(git.LocalBranches(), b)
	}

	switch cfg.Strategy {
	case flow.StrategyGitFlow:
		fmt.Printf("  %s%s\n", mark(cfg.MainBranch), ui.BoldCyan(cfg.MainBranch))
		fmt.Printf("  %s │\n", ui.Dim("  "))
		if exists(cfg.DevelopBranch) {
			fmt.Printf("  %s%s\n", mark(cfg.DevelopBranch), ui.Cyan(cfg.DevelopBranch))
		} else {
			fmt.Printf("  %s%s %s\n", "  ", ui.Dim(cfg.DevelopBranch), ui.Red("(없음)"))
		}
		for _, f := range cfg.ActiveFeatures() {
			fmt.Printf("  %s    └─ %s\n", ui.Dim("  "), ui.Yellow(cfg.FeatureBranch(f)))
		}
		for _, r := range cfg.ActiveReleases() {
			fmt.Printf("  %s    └─ %s\n", ui.Dim("  "), ui.Blue(cfg.ReleaseBranch(r)))
		}
		for _, h := range cfg.ActiveHotfixes() {
			fmt.Printf("  %s └─ %s\n", ui.Dim("  "), ui.Red(cfg.HotfixBranch(h)))
		}

	case flow.StrategyGitHubFlow:
		fmt.Printf("  %s%s\n", mark(cfg.MainBranch), ui.BoldCyan(cfg.MainBranch))
		for _, f := range cfg.ActiveFeatures() {
			fmt.Printf("  %s └─ %s\n", ui.Dim("  "), ui.Yellow(cfg.FeatureBranch(f)))
		}

	case flow.StrategyTrunk:
		fmt.Printf("  %s%s  %s\n", mark(cfg.MainBranch), ui.BoldCyan(cfg.MainBranch), ui.Dim("(trunk)"))
		for _, f := range cfg.ActiveFeatures() {
			fmt.Printf("  %s └─ %s %s\n", ui.Dim("  "), ui.Yellow(cfg.FeatureBranch(f)), ui.Dim("(short-lived)"))
		}
	}
	fmt.Println()
}

func printFlowHints(cfg *flow.Config, branchType string) {
	fmt.Printf("  %s\n", ui.Bold("다음 명령어:"))
	switch cfg.Strategy {
	case flow.StrategyGitFlow:
		switch branchType {
		case "main", "develop":
			fmt.Printf("    %s  새 feature 시작\n", ui.Cyan("gez flow feature start <이름>"))
			fmt.Printf("    %s  새 release 시작\n", ui.Cyan("gez flow release start <버전>"))
			fmt.Printf("    %s  긴급 hotfix 시작\n", ui.Cyan("gez flow hotfix start <이름>"))
		case "feature":
			fmt.Printf("    %s  feature 완료 → develop 머지\n", ui.Cyan("gez flow feature finish"))
			fmt.Printf("    %s  원격에 publish\n", ui.Cyan("gez flow feature publish"))
		case "release":
			fmt.Printf("    %s  release 완료 → main+develop 머지 + 태그\n", ui.Cyan("gez flow release finish"))
			fmt.Printf("    %s  원격에 publish\n", ui.Cyan("gez flow release publish"))
		case "hotfix":
			fmt.Printf("    %s  hotfix 완료 → main+develop 머지 + 태그\n", ui.Cyan("gez flow hotfix finish"))
		}
	case flow.StrategyGitHubFlow, flow.StrategyTrunk:
		switch branchType {
		case "main":
			fmt.Printf("    %s  새 feature 시작\n", ui.Cyan("gez flow feature start <이름>"))
		case "feature":
			fmt.Printf("    %s  feature 완료 → main 머지\n", ui.Cyan("gez flow feature finish"))
			fmt.Printf("    %s  원격에 publish (PR용)\n", ui.Cyan("gez flow feature publish"))
		}
	}
	fmt.Println()
}

func branchIcon(branchType string) string {
	switch branchType {
	case "main":
		return ui.BoldGreen("★")
	case "develop":
		return ui.BoldCyan("◆")
	case "feature":
		return ui.Yellow("◉")
	case "release":
		return ui.Blue("◉")
	case "hotfix":
		return ui.Red("◉")
	default:
		return ui.Dim("○")
	}
}

// ── init ──────────────────────────────────────────────────────────────────────

func init() {
	flowCmd.AddCommand(flowInitCmd)
	flowCmd.AddCommand(flowStatusCmd)
	rootCmd.AddCommand(flowCmd)
}

// ── helpers ───────────────────────────────────────────────────────────────────

func containsStr(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}
