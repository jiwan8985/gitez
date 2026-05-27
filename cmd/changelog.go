package cmd

import (
	"fmt"
	"gez/internal/git"
	"gez/internal/ui"
	"os"
	"strings"
	"time"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"
)

var changelogCmd = &cobra.Command{
	Use:   "changelog",
	Short: "Conventional Commits 기반 CHANGELOG 자동 생성",
	Run: func(cmd *cobra.Command, args []string) {
		if !git.IsRepo() {
			ui.Fail("git 저장소가 아닙니다")
			os.Exit(1)
		}
		runChangelog()
	},
}

func init() {
	rootCmd.AddCommand(changelogCmd)
}

// ccEntry represents a parsed conventional commit.
type ccEntry struct {
	hash     string
	ccType   string
	scope    string
	breaking bool
	subject  string
	raw      string
}

// CC type to section label mapping
var ccSections = []struct {
	ccType string
	label  string
}{
	{"feat", "✨ 새 기능 (Features)"},
	{"fix", "🐛 버그 수정 (Bug Fixes)"},
	{"perf", "⚡ 성능 개선 (Performance)"},
	{"refactor", "♻️  리팩토링 (Refactoring)"},
	{"docs", "📚 문서 (Documentation)"},
	{"test", "✅ 테스트 (Tests)"},
	{"build", "🏗️  빌드 (Build)"},
	{"ci", "🔧 CI"},
	{"chore", "🧹 기타 (Chore)"},
	{"revert", "⏪ 되돌리기 (Reverts)"},
}

func runChangelog() {
	fmt.Println()

	// Get tags for range selection
	tags := git.Tags()

	var fromRef, toRef string

	// Choose range
	var rangeChoice string
	options := []string{"마지막 태그 → HEAD (최신 변경사항)"}
	if len(tags) >= 2 {
		options = append(options, "태그 사이 범위 선택")
	}
	options = append(options, "전체 히스토리", "직접 범위 입력")

	if err := survey.AskOne(&survey.Select{
		Message: "CHANGELOG 범위:",
		Options: options,
	}, &rangeChoice); err != nil {
		return
	}

	switch {
	case strings.HasPrefix(rangeChoice, "마지막 태그"):
		if len(tags) > 0 {
			fromRef = tags[0]
		}
		toRef = "HEAD"

	case strings.HasPrefix(rangeChoice, "태그 사이"):
		if err := survey.AskOne(&survey.Select{
			Message: "시작 태그 (from):",
			Options: tags,
		}, &fromRef); err != nil {
			return
		}
		toOptions := append([]string{"HEAD"}, tags...)
		if err := survey.AskOne(&survey.Select{
			Message: "종료 태그 (to):",
			Options: toOptions,
		}, &toRef); err != nil {
			return
		}

	case strings.HasPrefix(rangeChoice, "전체"):
		fromRef = ""
		toRef = "HEAD"

	case strings.HasPrefix(rangeChoice, "직접"):
		_ = survey.AskOne(&survey.Input{
			Message: "시작 ref (from, 비워두면 전체):",
		}, &fromRef)
		if err := survey.AskOne(&survey.Input{
			Message: "종료 ref (to):",
			Default: "HEAD",
		}, &toRef); err != nil {
			return
		}
	}

	// Version label
	var version string
	if err := survey.AskOne(&survey.Input{
		Message: "버전 레이블 (예: v1.2.0):",
		Default: "Unreleased",
	}, &version); err != nil {
		return
	}

	// Collect commits
	logRange := toRef
	if fromRef != "" {
		logRange = fromRef + ".." + toRef
	}
	out, err := git.Run("log", "--format=%h|%s", "--no-merges", logRange)
	if err != nil || out == "" {
		ui.Info("해당 범위에 커밋이 없습니다")
		return
	}

	// Parse commits
	entries := parseConventionalCommits(strings.Split(out, "\n"))

	// Build changelog markdown
	var sb strings.Builder
	date := time.Now().Format("2006-01-02")
	sb.WriteString(fmt.Sprintf("## [%s] — %s\n\n", version, date))

	// Breaking changes first
	var breaking []ccEntry
	for _, e := range entries {
		if e.breaking {
			breaking = append(breaking, e)
		}
	}
	if len(breaking) > 0 {
		sb.WriteString("### ⚠ BREAKING CHANGES\n\n")
		for _, e := range breaking {
			sb.WriteString(fmt.Sprintf("- %s ([%s])\n", e.subject, e.hash))
		}
		sb.WriteString("\n")
	}

	// By section
	for _, sec := range ccSections {
		var section []ccEntry
		for _, e := range entries {
			if e.ccType == sec.ccType {
				section = append(section, e)
			}
		}
		if len(section) == 0 {
			continue
		}
		sb.WriteString(fmt.Sprintf("### %s\n\n", sec.label))
		for _, e := range section {
			scopePart := ""
			if e.scope != "" {
				scopePart = fmt.Sprintf("**%s**: ", e.scope)
			}
			sb.WriteString(fmt.Sprintf("- %s%s ([%s])\n", scopePart, e.subject, e.hash))
		}
		sb.WriteString("\n")
	}

	// Non-conventional commits
	var others []ccEntry
	for _, e := range entries {
		if e.ccType == "" {
			others = append(others, e)
		}
	}
	if len(others) > 0 {
		sb.WriteString("### 기타 변경사항\n\n")
		for _, e := range others {
			sb.WriteString(fmt.Sprintf("- %s ([%s])\n", e.raw, e.hash))
		}
		sb.WriteString("\n")
	}

	changelog := sb.String()

	// Display preview
	fmt.Println()
	fmt.Println(ui.Bold("  생성된 CHANGELOG:"))
	fmt.Println(ui.Dim(strings.Repeat("─", 60)))
	fmt.Println(changelog)
	fmt.Println(ui.Dim(strings.Repeat("─", 60)))

	// Output options
	var output string
	if err := survey.AskOne(&survey.Select{
		Message: "출력 방법:",
		Options: []string{
			"CHANGELOG.md 파일에 추가 (prepend)",
			"CHANGELOG.md 파일 덮어쓰기",
			"클립보드 복사 (표준출력)",
			"취소",
		},
	}, &output); err != nil || output == "취소" {
		return
	}

	fmt.Println()
	switch {
	case strings.HasPrefix(output, "CHANGELOG.md 파일에 추가"):
		existing := ""
		if data, err := os.ReadFile("CHANGELOG.md"); err == nil {
			existing = string(data)
		}
		if err := os.WriteFile("CHANGELOG.md", []byte(changelog+existing), 0644); err != nil {
			ui.Fail("파일 쓰기 실패: " + err.Error())
			return
		}
		ui.Success("CHANGELOG.md 업데이트 완료!")

	case strings.HasPrefix(output, "CHANGELOG.md 파일 덮어쓰기"):
		if err := os.WriteFile("CHANGELOG.md", []byte(changelog), 0644); err != nil {
			ui.Fail("파일 쓰기 실패: " + err.Error())
			return
		}
		ui.Success("CHANGELOG.md 생성 완료!")

	case strings.HasPrefix(output, "클립보드"):
		fmt.Println(changelog)
		ui.Info("위 내용을 복사하세요")
	}
	fmt.Println()
}

// parseConventionalCommits parses "hash|subject" lines.
func parseConventionalCommits(lines []string) []ccEntry {
	var result []ccEntry
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if l == "" {
			continue
		}
		parts := strings.SplitN(l, "|", 2)
		if len(parts) < 2 {
			continue
		}
		hash := parts[0]
		subject := parts[1]
		entry := ccEntry{hash: hash, raw: subject}

		// Parse conventional commit: type(scope)!: subject
		// Pattern: word chars, optional (scope), optional !, colon+space
		if idx := strings.Index(subject, ": "); idx > 0 {
			prefix := subject[:idx]
			rest := subject[idx+2:]

			breaking := strings.HasSuffix(prefix, "!")
			if breaking {
				prefix = strings.TrimSuffix(prefix, "!")
				entry.breaking = true
			}

			if idx2 := strings.Index(prefix, "("); idx2 >= 0 {
				entry.ccType = strings.ToLower(prefix[:idx2])
				entry.scope = strings.Trim(prefix[idx2:], "()")
			} else {
				entry.ccType = strings.ToLower(prefix)
			}

			// Validate type
			valid := false
			for _, sec := range ccSections {
				if entry.ccType == sec.ccType {
					valid = true
					break
				}
			}
			if valid {
				entry.subject = rest
			} else {
				entry.ccType = ""
			}
		}

		result = append(result, entry)
	}
	return result
}
