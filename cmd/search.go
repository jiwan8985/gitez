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

var searchCmd = &cobra.Command{
	Use:   "search",
	Short: "커밋 메시지·코드 변경 내용 검색",
	Run: func(cmd *cobra.Command, args []string) {
		if !git.IsRepo() {
			ui.Fail("git 저장소가 아닙니다")
			os.Exit(1)
		}
		runSearch()
	},
}

func init() {
	rootCmd.AddCommand(searchCmd)
}

func runSearch() {
	fmt.Println()
	var searchType string
	if err := survey.AskOne(&survey.Select{
		Message: "검색 유형:",
		Options: []string{
			"커밋 메시지 검색  (git log --grep)",
			"코드 변경 내용 검색  (git log -S, pickaxe)",
			"정규식으로 코드 검색  (git log -G)",
			"현재 파일에서 검색  (git grep)",
			"파일명 검색  (git log -- *pattern*)",
		},
	}, &searchType); err != nil {
		return
	}

	fmt.Println()
	switch {
	case strings.Contains(searchType, "--grep"):
		searchByMessage()
	case strings.Contains(searchType, "-S"):
		searchByPickaxe()
	case strings.Contains(searchType, "-G"):
		searchByRegex()
	case strings.Contains(searchType, "git grep"):
		searchInFiles()
	case strings.Contains(searchType, "파일명"):
		searchByFilename()
	}
}

func searchByMessage() {
	var query string
	if err := survey.AskOne(&survey.Input{
		Message: "검색어 (커밋 메시지):",
		Help:    "대소문자 구분 없음. 정규식 사용 가능",
	}, &query, survey.WithValidator(survey.Required)); err != nil {
		return
	}

	fmt.Println()
	ui.Info(fmt.Sprintf("메시지 검색: '%s'", query))
	fmt.Println()

	err := git.RunLive(
		"log",
		"--oneline",
		"--color=always",
		"--decorate",
		"-i",
		"--grep="+query,
		"-n", "50",
	)
	if err != nil {
		ui.Info("결과 없음")
	}
	fmt.Println()

	offerActionOnResult(query, "message")
}

func searchByPickaxe() {
	var query string
	if err := survey.AskOne(&survey.Input{
		Message: "검색 문자열 (추가/삭제된 코드):",
		Help:    "해당 문자열이 추가되거나 삭제된 커밋을 찾습니다",
	}, &query, survey.WithValidator(survey.Required)); err != nil {
		return
	}

	fmt.Println()
	ui.Info(fmt.Sprintf("코드 변경 검색: '%s'", query))
	fmt.Println()

	err := git.RunLive(
		"log",
		"--oneline",
		"--color=always",
		"-S", query,
		"-n", "50",
	)
	if err != nil {
		ui.Info("결과 없음")
	}
	fmt.Println()
}

func searchByRegex() {
	var pattern string
	if err := survey.AskOne(&survey.Input{
		Message: "정규식 패턴:",
		Help:    "변경 내용(diff)이 패턴에 매칭되는 커밋을 찾습니다",
	}, &pattern, survey.WithValidator(survey.Required)); err != nil {
		return
	}

	fmt.Println()
	ui.Info(fmt.Sprintf("정규식 검색: '%s'", pattern))
	fmt.Println()

	err := git.RunLive(
		"log",
		"--oneline",
		"--color=always",
		"-G", pattern,
		"-n", "50",
	)
	if err != nil {
		ui.Info("결과 없음")
	}
	fmt.Println()
}

func searchInFiles() {
	var pattern string
	if err := survey.AskOne(&survey.Input{
		Message: "검색 패턴 (현재 파일 기준):",
		Help:    "git grep — 현재 체크아웃된 파일에서 검색",
	}, &pattern, survey.WithValidator(survey.Required)); err != nil {
		return
	}

	fmt.Println()
	err := git.RunLive("grep", "--color=always", "-n", "-i", pattern)
	if err != nil {
		ui.Info("결과 없음")
	}
	fmt.Println()
}

func searchByFilename() {
	var pattern string
	if err := survey.AskOne(&survey.Input{
		Message: "파일명 패턴:",
		Help:    "예: *.go   auth.go   src/",
	}, &pattern, survey.WithValidator(survey.Required)); err != nil {
		return
	}

	fmt.Println()
	ui.Info(fmt.Sprintf("파일 관련 커밋 검색: '%s'", pattern))
	fmt.Println()

	err := git.RunLive(
		"log",
		"--oneline",
		"--color=always",
		"--follow",
		"-n", "50",
		"--", pattern,
	)
	if err != nil {
		ui.Info("결과 없음")
	}
	fmt.Println()
}

func offerActionOnResult(query, kind string) {
	var action string
	_ = survey.AskOne(&survey.Select{
		Message: "검색 결과로 작업:",
		Options: []string{
			"커밋 선택해서 상세 보기",
			"취소",
		},
	}, &action)

	if strings.HasPrefix(action, "커밋 선택") {
		// Re-collect results as selectable list
		var out string
		if kind == "message" {
			out, _ = git.Run("log", "--oneline", "-i", "--grep="+query, "-n", "50")
		}
		if out == "" {
			return
		}
		lines := strings.Split(strings.TrimSpace(out), "\n")
		var sel string
		if err := survey.AskOne(&survey.Select{
			Message: "커밋 선택:",
			Options: lines,
		}, &sel); err != nil {
			return
		}
		hash := strings.Fields(sel)[0]
		fmt.Println()
		_ = git.RunLive("show", "--stat", "--color=always", hash)
		fmt.Println()
	}
}
