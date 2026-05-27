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

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Git + gez 설정 조회/수정",
	Run: func(cmd *cobra.Command, args []string) {
		if !git.IsRepo() {
			ui.Fail("git 저장소가 아닙니다")
			os.Exit(1)
		}
		runConfigMenu()
	},
}

func init() {
	rootCmd.AddCommand(configCmd)
}

// Common git config keys to display/edit
var commonConfigKeys = []struct {
	key   string
	label string
	scope string // global / local
}{
	{"user.name", "사용자 이름", "global"},
	{"user.email", "이메일", "global"},
	{"core.editor", "기본 에디터", "global"},
	{"init.defaultBranch", "기본 브랜치 이름", "global"},
	{"pull.rebase", "pull 시 rebase 사용", "global"},
	{"core.autocrlf", "CRLF 자동 변환", "global"},
	{"push.autoSetupRemote", "push 자동 upstream 설정", "global"},
	{"merge.ff", "merge fast-forward 정책", "local"},
	{"branch.autosetuprebase", "브랜치 rebase 자동설정", "global"},
}

func runConfigMenu() {
	// Print current settings
	printConfigSummary()

	var action string
	if err := survey.AskOne(&survey.Select{
		Message: "작업 선택:",
		Options: []string{
			"Git 설정 수정",
			"gez flow 설정 보기",
			"전체 git config 보기 (local)",
			"전체 git config 보기 (global)",
			"취소",
		},
	}, &action); err != nil || action == "취소" {
		return
	}

	fmt.Println()
	switch {
	case strings.HasPrefix(action, "Git 설정 수정"):
		runConfigEdit()
	case strings.HasPrefix(action, "gez flow"):
		runFlowConfigView()
	case strings.Contains(action, "(local)"):
		_ = git.RunLive("config", "--local", "--list")
		fmt.Println()
	case strings.Contains(action, "(global)"):
		_ = git.RunLive("config", "--global", "--list")
		fmt.Println()
	}
}

func printConfigSummary() {
	fmt.Println()
	fmt.Printf("  %s\n\n", ui.Bold("현재 Git 설정"))

	sep := ui.Dim(strings.Repeat("─", 60))
	fmt.Println(sep)

	for _, entry := range commonConfigKeys {
		val, _ := git.Run("config", "--get", entry.key)
		val = strings.TrimSpace(val)
		if val == "" {
			val = ui.Dim("(미설정)")
		}
		scope := ui.Dim("[" + entry.scope + "]")
		fmt.Printf("  %-28s  %s  %s\n", ui.Cyan(entry.key), val, scope)
	}

	fmt.Println(sep)
	fmt.Println()
}

func runConfigEdit() {
	// Build display options
	options := make([]string, len(commonConfigKeys))
	for i, e := range commonConfigKeys {
		val, _ := git.Run("config", "--get", e.key)
		val = strings.TrimSpace(val)
		if val == "" {
			val = "(미설정)"
		}
		options[i] = fmt.Sprintf("%-28s = %s", e.key, val)
	}
	options = append(options, "직접 입력")

	var sel string
	if err := survey.AskOne(&survey.Select{
		Message: "수정할 설정:",
		Options: options,
	}, &sel); err != nil {
		return
	}

	var key, value string
	if sel == "직접 입력" {
		if err := survey.AskOne(&survey.Input{
			Message: "설정 키 (예: core.editor):",
		}, &key, survey.WithValidator(survey.Required)); err != nil {
			return
		}
	} else {
		key = strings.Fields(sel)[0]
	}

	current, _ := git.Run("config", "--get", key)
	if err := survey.AskOne(&survey.Input{
		Message: fmt.Sprintf("새 값 (%s):", key),
		Default: strings.TrimSpace(current),
	}, &value, survey.WithValidator(survey.Required)); err != nil {
		return
	}
	value = strings.TrimSpace(value)

	// Determine scope
	var scopeLabel string
	if err := survey.AskOne(&survey.Select{
		Message: "적용 범위:",
		Options: []string{"전역 (global) — 모든 프로젝트", "로컬 (local) — 이 저장소만"},
	}, &scopeLabel); err != nil {
		return
	}

	scope := "--global"
	if strings.HasPrefix(scopeLabel, "로컬") {
		scope = "--local"
	}

	fmt.Println()
	if _, err := git.Run("config", scope, key, value); err != nil {
		ui.Fail("설정 실패: " + err.Error())
		return
	}
	ui.Success(fmt.Sprintf("%s = %s  (%s)", key, value, strings.TrimPrefix(scope, "--")))
	fmt.Println()
}

func runFlowConfigView() {
	cfg, err := flow.Load()
	if err != nil {
		ui.Fail("flow 설정 로드 실패: " + err.Error())
		return
	}
	if !cfg.IsInitialised() {
		ui.Warn("flow 전략이 설정되지 않았습니다 — gez flow init 으로 설정하세요")
		return
	}

	fmt.Printf("  %s  %s\n\n", ui.Bold("Flow 전략:"), ui.BoldCyan(cfg.Strategy.Label()))

	entries := []struct{ k, v string }{
		{"main 브랜치", cfg.MainBranch},
		{"develop 브랜치", cfg.DevelopBranch},
		{"feature prefix", cfg.FeaturePrefix},
		{"release prefix", cfg.ReleasePrefix},
		{"hotfix prefix", cfg.HotfixPrefix},
		{"support prefix", cfg.SupportPrefix},
		{"tag prefix", cfg.TagPrefix},
	}
	for _, e := range entries {
		if e.v != "" {
			fmt.Printf("  %-20s  %s\n", ui.Dim(e.k), ui.Cyan(e.v))
		}
	}
	fmt.Println()

	var edit bool
	_ = survey.AskOne(&survey.Confirm{
		Message: "flow 설정을 다시 초기화하겠습니까?",
		Default: false,
	}, &edit)
	if edit {
		runFlowInit()
	}
}
