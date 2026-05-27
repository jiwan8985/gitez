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

var aliasCmd = &cobra.Command{
	Use:   "alias",
	Short: "Git 별칭(alias) 관리 (목록·추가·삭제)",
	Run: func(cmd *cobra.Command, args []string) {
		if !git.IsRepo() {
			ui.Fail("git 저장소가 아닙니다")
			os.Exit(1)
		}
		runAliasMenu()
	},
}

var aliasListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "등록된 git alias 목록",
	Run:     func(cmd *cobra.Command, args []string) { printAliasList() },
}

var aliasAddCmd = &cobra.Command{
	Use:   "add <이름> <명령>",
	Short: "git alias 추가",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		doAliasAdd(args[0], args[1], "global")
	},
}

var aliasRmCmd = &cobra.Command{
	Use:     "rm <이름>",
	Short:   "git alias 삭제",
	Aliases: []string{"remove", "delete"},
	Args:    cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		doAliasRemove(args[0])
	},
}

func init() {
	aliasCmd.AddCommand(aliasListCmd)
	aliasCmd.AddCommand(aliasAddCmd)
	aliasCmd.AddCommand(aliasRmCmd)
	rootCmd.AddCommand(aliasCmd)
}

// Preset aliases
var presetAliases = []struct {
	name, command, desc string
}{
	{"st", "status --short --branch", "짧은 status"},
	{"co", "checkout", "checkout 단축"},
	{"br", "branch -vv", "브랜치 + upstream 표시"},
	{"lg", "log --oneline --graph --decorate --all", "그래프 로그"},
	{"last", "log -1 HEAD --stat", "마지막 커밋"},
	{"unstage", "restore --staged", "unstage 파일"},
	{"undo", "reset --soft HEAD~1", "마지막 커밋 취소"},
	{"aliases", "config --get-regexp ^alias", "alias 목록"},
	{"contributors", "shortlog -sn --no-merges", "기여자 목록"},
	{"stash-list", "stash list --format='%gd: %Cred%h%Creset %s'", "stash 목록"},
}

func runAliasMenu() {
	printAliasList()

	var action string
	if err := survey.AskOne(&survey.Select{
		Message: "작업 선택:",
		Options: []string{
			"alias 추가 (직접 입력)",
			"프리셋 alias 설치",
			"alias 삭제",
			"alias 실행 테스트",
			"취소",
		},
	}, &action); err != nil || action == "취소" {
		return
	}

	fmt.Println()
	switch {
	case strings.HasPrefix(action, "alias 추가"):
		var name, command string
		if err := survey.AskOne(&survey.Input{
			Message: "alias 이름:",
			Help:    "예: st, lg, undo",
		}, &name, survey.WithValidator(survey.Required)); err != nil {
			return
		}
		if err := survey.AskOne(&survey.Input{
			Message: "git 명령어:",
			Help:    "예: status --short   log --oneline -20",
		}, &command, survey.WithValidator(survey.Required)); err != nil {
			return
		}
		var scope string
		_ = survey.AskOne(&survey.Select{
			Message: "적용 범위:",
			Options: []string{"global (전체)", "local (이 저장소)"},
		}, &scope)
		scopeFlag := "global"
		if strings.HasPrefix(scope, "local") {
			scopeFlag = "local"
		}
		doAliasAdd(strings.TrimSpace(name), strings.TrimSpace(command), scopeFlag)

	case strings.HasPrefix(action, "프리셋"):
		var options []string
		for _, p := range presetAliases {
			options = append(options, fmt.Sprintf("%-16s  %s  (%s)", p.name, p.desc, p.command))
		}
		var selected []string
		if err := survey.AskOne(&survey.MultiSelect{
			Message: "설치할 프리셋 선택:",
			Options: options,
		}, &selected); err != nil || len(selected) == 0 {
			return
		}
		for _, sel := range selected {
			name := strings.Fields(sel)[0]
			for _, p := range presetAliases {
				if p.name == name {
					doAliasAdd(p.name, p.command, "global")
					break
				}
			}
		}

	case strings.HasPrefix(action, "alias 삭제"):
		aliases := getAliasList()
		if len(aliases) == 0 {
			ui.Info("등록된 alias가 없습니다")
			return
		}
		var options []string
		for _, a := range aliases {
			options = append(options, a)
		}
		var selected []string
		if err := survey.AskOne(&survey.MultiSelect{
			Message: "삭제할 alias 선택:",
			Options: options,
		}, &selected); err != nil || len(selected) == 0 {
			return
		}
		for _, s := range selected {
			name := strings.Fields(s)[0]
			doAliasRemove(name)
		}

	case strings.HasPrefix(action, "alias 실행 테스트"):
		aliases := getAliasList()
		if len(aliases) == 0 {
			ui.Info("등록된 alias가 없습니다")
			return
		}
		var sel string
		if err := survey.AskOne(&survey.Select{
			Message: "실행할 alias:",
			Options: aliases,
		}, &sel); err != nil {
			return
		}
		name := strings.Fields(sel)[0]
		fmt.Println()
		_ = git.RunLive(name)
		fmt.Println()
	}
}

func printAliasList() {
	aliases := getAliasList()
	fmt.Println()
	if len(aliases) == 0 {
		fmt.Printf("  %s\n\n", ui.Dim("등록된 git alias가 없습니다"))
		return
	}
	fmt.Printf("  %s  (%d개)\n\n", ui.Bold("Git Aliases"), len(aliases))
	for _, a := range aliases {
		parts := strings.SplitN(a, "=", 2)
		if len(parts) == 2 {
			fmt.Printf("  %s  =  %s\n", ui.Cyan(parts[0]), ui.Dim(parts[1]))
		} else {
			fmt.Printf("  %s\n", a)
		}
	}
	fmt.Println()
}

func getAliasList() []string {
	out, err := git.Run("config", "--get-regexp", "^alias\\.")
	if err != nil || out == "" {
		return nil
	}
	var result []string
	for _, l := range strings.Split(out, "\n") {
		l = strings.TrimSpace(l)
		if l == "" {
			continue
		}
		// "alias.st status --short" -> "st=status --short"
		parts := strings.SplitN(l, " ", 2)
		if len(parts) == 2 {
			name := strings.TrimPrefix(parts[0], "alias.")
			result = append(result, name+"="+parts[1])
		}
	}
	return result
}

func doAliasAdd(name, command, scope string) {
	key := "alias." + name
	flag := "--" + scope
	if _, err := git.Run("config", flag, key, command); err != nil {
		ui.Fail(fmt.Sprintf("'%s' 추가 실패: %s", name, err.Error()))
		return
	}
	ui.Success(fmt.Sprintf("alias 추가: %s = %s  [%s]", ui.Cyan(name), command, scope))
}

func doAliasRemove(name string) {
	key := "alias." + name
	// Try global first, then local
	_, errG := git.Run("config", "--global", "--unset", key)
	_, errL := git.Run("config", "--local", "--unset", key)
	if errG != nil && errL != nil {
		ui.Fail(fmt.Sprintf("'%s' alias를 찾을 수 없습니다", name))
		return
	}
	ui.Success(fmt.Sprintf("alias '%s' 삭제 완료", name))
}
