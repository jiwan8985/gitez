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

var remoteCmd = &cobra.Command{
	Use:   "remote",
	Short: "원격 저장소 관리 (목록·추가·삭제·URL 변경)",
	Run: func(cmd *cobra.Command, args []string) {
		if !git.IsRepo() {
			ui.Fail("git 저장소가 아닙니다")
			os.Exit(1)
		}
		runRemoteMenu()
	},
}

var remoteListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "원격 저장소 목록",
	Run: func(cmd *cobra.Command, args []string) {
		if !git.IsRepo() {
			ui.Fail("git 저장소가 아닙니다")
			os.Exit(1)
		}
		printRemotes()
	},
}

var remoteAddCmd = &cobra.Command{
	Use:   "add <이름> <URL>",
	Short: "원격 저장소 추가",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		if !git.IsRepo() {
			ui.Fail("git 저장소가 아닙니다")
			os.Exit(1)
		}
		doRemoteAdd(args[0], args[1])
	},
}

var remoteRmCmd = &cobra.Command{
	Use:     "rm <이름>",
	Aliases: []string{"remove", "del"},
	Short:   "원격 저장소 제거",
	Args:    cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if !git.IsRepo() {
			ui.Fail("git 저장소가 아닙니다")
			os.Exit(1)
		}
		doRemoteRemove(args[0])
	},
}

var remoteSetUrlCmd = &cobra.Command{
	Use:   "set-url <이름> <URL>",
	Short: "원격 저장소 URL 변경",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		if !git.IsRepo() {
			ui.Fail("git 저장소가 아닙니다")
			os.Exit(1)
		}
		doRemoteSetURL(args[0], args[1])
	},
}

func init() {
	remoteCmd.AddCommand(remoteListCmd)
	remoteCmd.AddCommand(remoteAddCmd)
	remoteCmd.AddCommand(remoteRmCmd)
	remoteCmd.AddCommand(remoteSetUrlCmd)
	rootCmd.AddCommand(remoteCmd)
}

// ── interactive menu ──────────────────────────────────────────────────────────

func runRemoteMenu() {
	printRemotes()

	remotes := git.Remotes()
	actions := []string{"원격 저장소 추가"}
	if len(remotes) > 0 {
		actions = append(actions, "원격 저장소 URL 변경")
		actions = append(actions, "원격 저장소 제거")
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
	case strings.HasPrefix(action, "원격 저장소 추가"):
		runRemoteAddInteractive()

	case strings.HasPrefix(action, "원격 저장소 URL 변경"):
		if r := selectRemote(remotes, "URL을 변경할 원격 선택:"); r != "" {
			var newURL string
			if err := survey.AskOne(&survey.Input{
				Message: fmt.Sprintf("'%s' 의 새 URL:", r),
			}, &newURL, survey.WithValidator(survey.Required)); err == nil {
				doRemoteSetURL(r, strings.TrimSpace(newURL))
			}
		}

	case strings.HasPrefix(action, "원격 저장소 제거"):
		if r := selectRemote(remotes, "제거할 원격 선택:"); r != "" {
			doRemoteRemove(r)
		}
	}
}

// ── helpers ───────────────────────────────────────────────────────────────────

func printRemotes() {
	remotes := git.Remotes()
	fmt.Println()
	if len(remotes) == 0 {
		fmt.Printf("  %s\n\n", ui.Dim("등록된 원격 저장소가 없습니다"))
		return
	}
	fmt.Printf("  %s\n", ui.Bold("원격 저장소:"))
	for _, r := range remotes {
		if r == "" {
			continue
		}
		fetchURL, _ := git.Run("remote", "get-url", r)
		fmt.Printf("    %s  %s\n", ui.BoldCyan(r), ui.Dim(fetchURL))
	}
	fmt.Println()
}

func runRemoteAddInteractive() {
	var name string
	if err := survey.AskOne(&survey.Input{
		Message: "원격 이름:",
		Default: "origin",
	}, &name, survey.WithValidator(survey.Required)); err != nil {
		return
	}
	var url string
	if err := survey.AskOne(&survey.Input{
		Message: "URL:",
		Help:    "예: https://github.com/user/repo.git  또는  git@github.com:user/repo.git",
	}, &url, survey.WithValidator(survey.Required)); err != nil {
		return
	}
	doRemoteAdd(strings.TrimSpace(name), strings.TrimSpace(url))
}

func doRemoteAdd(name, url string) {
	if _, err := git.Run("remote", "add", name, url); err != nil {
		ui.Fail("원격 추가 실패: " + err.Error())
		return
	}
	ui.Success(fmt.Sprintf("원격 '%s' 추가 완료  (%s)", name, url))
}

func doRemoteRemove(name string) {
	var ok bool
	if err := survey.AskOne(&survey.Confirm{
		Message: fmt.Sprintf("원격 '%s' 를 제거할까요?", name),
		Default: false,
	}, &ok); err != nil || !ok {
		ui.Warn("취소되었습니다")
		return
	}
	if _, err := git.Run("remote", "remove", name); err != nil {
		ui.Fail("원격 제거 실패: " + err.Error())
		return
	}
	ui.Success(fmt.Sprintf("원격 '%s' 제거 완료", name))
}

func doRemoteSetURL(name, url string) {
	if _, err := git.Run("remote", "set-url", name, url); err != nil {
		ui.Fail("URL 변경 실패: " + err.Error())
		return
	}
	ui.Success(fmt.Sprintf("'%s' URL 변경 완료  →  %s", name, url))
}

func selectRemote(remotes []string, prompt string) string {
	var filtered []string
	for _, r := range remotes {
		if r != "" {
			filtered = append(filtered, r)
		}
	}
	if len(filtered) == 0 {
		return ""
	}
	var selected string
	if err := survey.AskOne(&survey.Select{
		Message: prompt,
		Options: filtered,
	}, &selected); err != nil {
		return ""
	}
	return selected
}
