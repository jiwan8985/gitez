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

var tagCmd = &cobra.Command{
	Use:   "tag",
	Short: "태그 관리 (목록·생성·삭제·push)",
	Run: func(cmd *cobra.Command, args []string) {
		if !git.IsRepo() {
			ui.Fail("git 저장소가 아닙니다")
			os.Exit(1)
		}
		runTagMenu()
	},
}

var tagListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "태그 목록 보기",
	Run: func(cmd *cobra.Command, args []string) {
		if !git.IsRepo() {
			ui.Fail("git 저장소가 아닙니다")
			os.Exit(1)
		}
		printTags()
	},
}

var tagCreateCmd = &cobra.Command{
	Use:   "create <이름> [메시지]",
	Short: "새 태그 생성",
	Args:  cobra.RangeArgs(1, 2),
	Run: func(cmd *cobra.Command, args []string) {
		if !git.IsRepo() {
			ui.Fail("git 저장소가 아닙니다")
			os.Exit(1)
		}
		msg := ""
		if len(args) > 1 {
			msg = args[1]
		}
		doTagCreate(args[0], msg)
	},
}

var tagDeleteCmd = &cobra.Command{
	Use:     "delete <이름>",
	Aliases: []string{"del", "rm"},
	Short:   "태그 삭제 (로컬 + 원격)",
	Args:    cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if !git.IsRepo() {
			ui.Fail("git 저장소가 아닙니다")
			os.Exit(1)
		}
		doTagDelete(args[0])
	},
}

var tagPushCmd = &cobra.Command{
	Use:   "push [이름]",
	Short: "태그를 원격에 push (이름 없으면 전체)",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if !git.IsRepo() {
			ui.Fail("git 저장소가 아닙니다")
			os.Exit(1)
		}
		if len(args) > 0 {
			doTagPushOne(args[0])
		} else {
			doTagPushAll()
		}
	},
}

func init() {
	tagCmd.AddCommand(tagListCmd)
	tagCmd.AddCommand(tagCreateCmd)
	tagCmd.AddCommand(tagDeleteCmd)
	tagCmd.AddCommand(tagPushCmd)
	rootCmd.AddCommand(tagCmd)
}

// ── interactive menu ──────────────────────────────────────────────────────────

func runTagMenu() {
	printTags()

	tags := git.Tags()
	actions := []string{"새 태그 생성"}
	if len(tags) > 0 {
		actions = append(actions, "태그 삭제")
		actions = append(actions, "태그 원격에 push")
		actions = append(actions, "모든 태그 원격에 push")
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
	case strings.HasPrefix(action, "새 태그"):
		runTagCreateInteractive()

	case strings.HasPrefix(action, "태그 삭제"):
		if tag := selectTag(tags, "삭제할 태그 선택:"); tag != "" {
			doTagDelete(tag)
		}

	case strings.HasPrefix(action, "태그 원격에 push") && !strings.HasPrefix(action, "모든"):
		if tag := selectTag(tags, "push할 태그 선택:"); tag != "" {
			doTagPushOne(tag)
		}

	case strings.HasPrefix(action, "모든 태그"):
		doTagPushAll()
	}
}

// ── helpers ───────────────────────────────────────────────────────────────────

func printTags() {
	tags := git.Tags()
	fmt.Println()
	if len(tags) == 0 {
		fmt.Printf("  %s\n\n", ui.Dim("태그가 없습니다"))
		return
	}
	fmt.Printf("  %s  (%d개)\n", ui.Bold("태그 목록"), len(tags))
	for _, t := range tags {
		if t == "" {
			continue
		}
		// Try to get the tagged commit info
		info, _ := git.Run("log", "-1", "--pretty=%h  %s", t)
		if info != "" {
			fmt.Printf("    %s  %s\n", ui.Yellow(t), ui.Dim(info))
		} else {
			fmt.Printf("    %s\n", ui.Yellow(t))
		}
	}
	fmt.Println()
}

func runTagCreateInteractive() {
	var name string
	if err := survey.AskOne(&survey.Input{
		Message: "태그 이름:",
		Help:    "예: v1.0.0  v2.3-beta  release-2024",
	}, &name, survey.WithValidator(survey.Required)); err != nil {
		return
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return
	}

	var msg string
	_ = survey.AskOne(&survey.Input{
		Message: "태그 메시지 (비워두면 lightweight 태그):",
	}, &msg)

	doTagCreate(name, strings.TrimSpace(msg))
}

func doTagCreate(name, msg string) {
	var err error
	if msg != "" {
		_, err = git.Run("tag", "-a", name, "-m", msg)
	} else {
		_, err = git.Run("tag", name)
	}

	if err != nil {
		ui.Fail("태그 생성 실패: " + err.Error())
		return
	}

	if msg != "" {
		ui.Success(fmt.Sprintf("Annotated 태그 '%s' 생성 완료", name))
	} else {
		ui.Success(fmt.Sprintf("Lightweight 태그 '%s' 생성 완료", name))
	}

	var push bool
	if err := survey.AskOne(&survey.Confirm{
		Message: "원격에 바로 push할까요?",
		Default: true,
	}, &push); err == nil && push {
		doTagPushOne(name)
	}
}

func doTagDelete(name string) {
	var ok bool
	if err := survey.AskOne(&survey.Confirm{
		Message: fmt.Sprintf("태그 '%s' 를 삭제할까요?", name),
		Default: false,
	}, &ok); err != nil || !ok {
		ui.Warn("취소되었습니다")
		return
	}

	// Delete locally
	if _, err := git.Run("tag", "-d", name); err != nil {
		ui.Fail("로컬 태그 삭제 실패: " + err.Error())
		return
	}
	ui.Success(fmt.Sprintf("로컬 태그 '%s' 삭제 완료", name))

	// Ask about remote
	var delRemote bool
	if err := survey.AskOne(&survey.Confirm{
		Message: "원격(origin)에서도 태그를 삭제할까요?",
		Default: false,
	}, &delRemote); err == nil && delRemote {
		if err := git.RunLive("push", "origin", ":refs/tags/"+name); err != nil {
			ui.Warn("원격 태그 삭제 실패")
			return
		}
		ui.Success(fmt.Sprintf("원격 태그 '%s' 삭제 완료", name))
	}
}

func doTagPushOne(name string) {
	fmt.Println()
	if err := git.RunLive("push", "origin", name); err != nil {
		ui.Fail("태그 push 실패")
		return
	}
	ui.Success(fmt.Sprintf("태그 '%s' push 완료!", name))
	fmt.Println()
}

func doTagPushAll() {
	fmt.Println()
	if err := git.RunLive("push", "origin", "--tags"); err != nil {
		ui.Fail("태그 push 실패")
		return
	}
	ui.Success("모든 태그 push 완료!")
	fmt.Println()
}

func selectTag(tags []string, prompt string) string {
	var filtered []string
	for _, t := range tags {
		if t != "" {
			filtered = append(filtered, t)
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
