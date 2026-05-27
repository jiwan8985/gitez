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

var commitCmd = &cobra.Command{
	Use:     "commit",
	Aliases: []string{"c"},
	Short:   "대화형 커밋 마법사 (스테이징 → 메시지 → push)",
	Run: func(cmd *cobra.Command, args []string) {
		if !git.IsRepo() {
			ui.Fail("git 저장소가 아닙니다")
			os.Exit(1)
		}
		runCommit()
	},
}

func init() {
	rootCmd.AddCommand(commitCmd)
}

// Conventional Commits types
var ccTypes = []string{
	"feat     — 새 기능",
	"fix      — 버그 수정",
	"docs     — 문서 변경",
	"style    — 코드 형식 (동작 변경 없음)",
	"refactor — 리팩토링",
	"perf     — 성능 개선",
	"test     — 테스트 추가/수정",
	"build    — 빌드 시스템, 외부 의존성",
	"ci       — CI 설정 변경",
	"chore    — 기타 잡다한 변경",
	"revert   — 이전 커밋 되돌리기",
	"직접 입력",
}

func runCommit() {
	lines := git.StatusShort()
	if len(lines) == 0 {
		ui.Info("커밋할 변경사항이 없습니다")
		return
	}

	// ── 현재 상태 출력 ──────────────────────────────────────────
	fmt.Println()
	fmt.Println(ui.Bold("  현재 변경사항:"))
	for _, l := range lines {
		if len(l) < 3 {
			continue
		}
		fmt.Printf("    %s  %s\n", ui.ColorXY(l[:2]), l[3:])
	}
	fmt.Println()

	// ── 스테이징 ────────────────────────────────────────────────
	var unstagedFiles []string
	for _, l := range lines {
		if len(l) < 3 {
			continue
		}
		y := l[1]
		if y != ' ' {
			unstagedFiles = append(unstagedFiles, l[3:])
		}
	}

	hasStagedAlready := false
	for _, l := range lines {
		if len(l) >= 2 && l[0] != ' ' && l[0] != '?' {
			hasStagedAlready = true
			break
		}
	}

	if len(unstagedFiles) > 0 {
		stageOptions := []string{"모두 스테이징  (git add -A)"}
		if len(unstagedFiles) > 1 {
			stageOptions = append(stageOptions, "파일 선택해서 스테이징")
		}
		if hasStagedAlready {
			stageOptions = append(stageOptions, "이미 스테이징된 것만 커밋")
		}
		stageOptions = append(stageOptions, "취소")

		var stageChoice string
		if err := survey.AskOne(&survey.Select{
			Message: "스테이징 방법:",
			Options: stageOptions,
		}, &stageChoice); err != nil || stageChoice == "취소" {
			ui.Warn("취소되었습니다")
			return
		}

		switch {
		case strings.HasPrefix(stageChoice, "모두 스테이징"):
			if _, err := git.Run("add", "-A"); err != nil {
				ui.Fail("스테이징 실패: " + err.Error())
				return
			}
			ui.Success("모든 변경사항 스테이징 완료")

		case stageChoice == "파일 선택해서 스테이징":
			var selected []string
			if err := survey.AskOne(&survey.MultiSelect{
				Message: "스테이징할 파일 선택 (Space=선택, Enter=확인):",
				Options: unstagedFiles,
			}, &selected); err != nil || len(selected) == 0 {
				ui.Warn("선택된 파일이 없습니다")
				return
			}
			for _, f := range selected {
				if _, err := git.Run("add", f); err != nil {
					ui.Fail(fmt.Sprintf("스테이징 실패 (%s): %s", f, err.Error()))
					return
				}
			}
			ui.Success(fmt.Sprintf("%d개 파일 스테이징 완료", len(selected)))

		case strings.HasPrefix(stageChoice, "이미 스테이징"):
			// nothing to do
		}
	}

	// ── 스테이징 확인 ───────────────────────────────────────────
	staged, _ := git.Run("diff", "--cached", "--name-only")
	if strings.TrimSpace(staged) == "" {
		ui.Warn("스테이징된 파일이 없어 커밋할 수 없습니다")
		return
	}

	fmt.Println()
	fmt.Println(ui.Bold("  커밋될 파일:"))
	for _, f := range strings.Split(staged, "\n") {
		if f != "" {
			fmt.Printf("    %s  %s\n", ui.Green("✔"), f)
		}
	}
	fmt.Println()

	// ── Conventional Commits 타입 선택 ─────────────────────────
	var useCC bool
	_ = survey.AskOne(&survey.Confirm{
		Message: "Conventional Commits 형식 사용? (feat:, fix:, ...)",
		Default: false,
	}, &useCC)

	prefix := ""
	if useCC {
		var ccChoice string
		if err := survey.AskOne(&survey.Select{
			Message: "커밋 타입:",
			Options: ccTypes,
		}, &ccChoice); err != nil {
			return
		}

		if ccChoice == "직접 입력" {
			var custom string
			if err := survey.AskOne(&survey.Input{
				Message: "타입 입력 (예: feat, fix):",
			}, &custom, survey.WithValidator(survey.Required)); err != nil {
				return
			}
			prefix = strings.TrimSpace(custom)
		} else {
			prefix = strings.Fields(ccChoice)[0]
		}

		// Optional scope
		var scope string
		_ = survey.AskOne(&survey.Input{
			Message: "스코프 (선택사항, 예: auth, ui, db):",
			Help:    "Enter 건너뛰기",
		}, &scope)
		scope = strings.TrimSpace(scope)

		// Optional breaking change
		var breaking bool
		_ = survey.AskOne(&survey.Confirm{
			Message: "Breaking change?",
			Default: false,
		}, &breaking)

		if scope != "" {
			prefix = fmt.Sprintf("%s(%s)", prefix, scope)
		}
		if breaking {
			prefix = prefix + "!"
		}
		prefix = prefix + ": "
	}

	// ── 커밋 메시지 ─────────────────────────────────────────────
	var subject string
	prompt := "커밋 메시지:"
	if prefix != "" {
		prompt = fmt.Sprintf("커밋 메시지 (%s):", ui.Cyan(prefix))
	}
	if err := survey.AskOne(&survey.Input{
		Message: prompt,
	}, &subject, survey.WithValidator(survey.Required)); err != nil {
		ui.Warn("취소되었습니다")
		return
	}
	subject = strings.TrimSpace(subject)
	if subject == "" {
		ui.Warn("메시지가 비어있어 취소되었습니다")
		return
	}

	// Optional body
	var body string
	_ = survey.AskOne(&survey.Input{
		Message: "본문 (선택사항, Enter 건너뛰기):",
	}, &body)

	fullMsg := prefix + subject
	if strings.TrimSpace(body) != "" {
		fullMsg = fullMsg + "\n\n" + strings.TrimSpace(body)
	}

	// ── 커밋 실행 ───────────────────────────────────────────────
	if _, err := git.Run("commit", "-m", fullMsg); err != nil {
		ui.Fail("커밋 실패: " + err.Error())
		return
	}

	hash, _ := git.Run("rev-parse", "--short", "HEAD")
	branch := git.CurrentBranch()
	fmt.Println()
	ui.Success(fmt.Sprintf("커밋 완료! [%s %s] %s", ui.Cyan(branch), ui.Yellow(hash), subject))
	fmt.Println()

	// ── push 여부 ────────────────────────────────────────────────
	var doPush bool
	if err := survey.AskOne(&survey.Confirm{
		Message: "원격 저장소에 푸시할까요?",
		Default: true,
	}, &doPush); err == nil && doPush {
		doPushBranch(branch, false)
	}
}
