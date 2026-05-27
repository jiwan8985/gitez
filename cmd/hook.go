package cmd

import (
	"fmt"
	"gez/internal/git"
	"gez/internal/ui"
	"os"
	"path/filepath"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"
)

var hookCmd = &cobra.Command{
	Use:   "hook",
	Short: "Git hooks 관리 (목록·활성화·비활성화·편집·프리셋 설치)",
	Run: func(cmd *cobra.Command, args []string) {
		if !git.IsRepo() {
			ui.Fail("git 저장소가 아닙니다")
			os.Exit(1)
		}
		runHookMenu()
	},
}

var hookListCmd = &cobra.Command{
	Use:   "list",
	Short: "hooks 목록 및 상태 보기",
	Run:   func(cmd *cobra.Command, args []string) { printHookList() },
}

var hookEnableCmd = &cobra.Command{
	Use:   "enable <hook>",
	Short: "hook 활성화 (.sample 제거 또는 실행권한 부여)",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) { doHookEnable(args[0]) },
}

var hookDisableCmd = &cobra.Command{
	Use:   "disable <hook>",
	Short: "hook 비활성화 (.sample 추가)",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) { doHookDisable(args[0]) },
}

var hookInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "프리셋 hook 설치 (conventional-commit-msg, pre-commit 등)",
	Run:   func(cmd *cobra.Command, args []string) { runHookInstall() },
}

func init() {
	hookCmd.AddCommand(hookListCmd)
	hookCmd.AddCommand(hookEnableCmd)
	hookCmd.AddCommand(hookDisableCmd)
	hookCmd.AddCommand(hookInstallCmd)
	rootCmd.AddCommand(hookCmd)
}

// Known hook names (git built-in hooks)
var knownHooks = []string{
	"pre-commit",
	"prepare-commit-msg",
	"commit-msg",
	"post-commit",
	"pre-push",
	"pre-rebase",
	"post-checkout",
	"post-merge",
	"post-rewrite",
	"pre-receive",
	"update",
	"post-receive",
	"post-update",
}

// hookState represents a hook file's state.
type hookState struct {
	name    string
	path    string
	active  bool // executable and not .sample
	sample  bool // ends in .sample
	exists  bool
}

func getHooksDir() string {
	dir, err := git.Run("rev-parse", "--git-dir")
	if err != nil {
		return ""
	}
	return filepath.Join(strings.TrimSpace(dir), "hooks")
}

func loadHookStates() []hookState {
	hooksDir := getHooksDir()
	if hooksDir == "" {
		return nil
	}

	var states []hookState
	for _, name := range knownHooks {
		path := filepath.Join(hooksDir, name)
		samplePath := path + ".sample"

		state := hookState{name: name}

		if info, err := os.Stat(path); err == nil {
			state.path = path
			state.exists = true
			state.active = info.Mode()&0111 != 0
		} else if _, err := os.Stat(samplePath); err == nil {
			state.path = samplePath
			state.sample = true
		}
		states = append(states, state)
	}
	return states
}

// ── 대화형 메뉴 ───────────────────────────────────────────────────────────────

func runHookMenu() {
	printHookList()

	var action string
	if err := survey.AskOne(&survey.Select{
		Message: "작업 선택:",
		Options: []string{
			"hook 활성화",
			"hook 비활성화",
			"프리셋 hook 설치",
			"취소",
		},
	}, &action); err != nil || action == "취소" {
		return
	}

	fmt.Println()
	switch {
	case strings.HasPrefix(action, "hook 활성화"):
		states := loadHookStates()
		var inactive []string
		for _, s := range states {
			if !s.active {
				inactive = append(inactive, s.name)
			}
		}
		if len(inactive) == 0 {
			ui.Info("모든 hook이 이미 활성화됐습니다")
			return
		}
		var sel string
		if err := survey.AskOne(&survey.Select{
			Message: "활성화할 hook:",
			Options: inactive,
		}, &sel); err != nil {
			return
		}
		doHookEnable(sel)

	case strings.HasPrefix(action, "hook 비활성화"):
		states := loadHookStates()
		var active []string
		for _, s := range states {
			if s.active {
				active = append(active, s.name)
			}
		}
		if len(active) == 0 {
			ui.Info("활성화된 hook이 없습니다")
			return
		}
		var sel string
		if err := survey.AskOne(&survey.Select{
			Message: "비활성화할 hook:",
			Options: active,
		}, &sel); err != nil {
			return
		}
		doHookDisable(sel)

	case strings.HasPrefix(action, "프리셋"):
		runHookInstall()
	}
}

// ── 실행 로직 ─────────────────────────────────────────────────────────────────

func printHookList() {
	states := loadHookStates()
	fmt.Println()
	fmt.Printf("  %s\n\n", ui.Bold("Git Hooks 상태"))

	sep := ui.Dim(strings.Repeat("─", 48))
	fmt.Println(sep)

	for _, s := range states {
		var icon, status string
		switch {
		case s.active:
			icon = ui.Green("✔")
			status = ui.Green("활성")
		case s.sample:
			icon = ui.Dim("○")
			status = ui.Dim("샘플만")
		case s.exists:
			icon = ui.Yellow("⚠")
			status = ui.Yellow("비실행")
		default:
			icon = ui.Dim("·")
			status = ui.Dim("없음")
		}
		fmt.Printf("  %s  %-28s  %s\n", icon, s.name, status)
	}
	fmt.Println(sep)
	fmt.Println()
}

func doHookEnable(name string) {
	hooksDir := getHooksDir()
	hookPath := filepath.Join(hooksDir, name)
	samplePath := hookPath + ".sample"

	// If only sample exists, copy it to active
	if _, err := os.Stat(hookPath); os.IsNotExist(err) {
		if _, err := os.Stat(samplePath); err == nil {
			// Read sample content
			content, err := os.ReadFile(samplePath)
			if err != nil {
				ui.Fail("샘플 파일 읽기 실패")
				return
			}
			if err := os.WriteFile(hookPath, content, 0755); err != nil {
				ui.Fail("hook 파일 생성 실패")
				return
			}
			ui.Success(fmt.Sprintf("'%s' hook 활성화 완료 (샘플에서 복사)", name))
			return
		}
		ui.Warn(fmt.Sprintf("'%s' hook 파일이 없습니다. 먼저 hook 스크립트를 작성하세요", name))
		ui.Info(fmt.Sprintf("위치: %s", hookPath))
		return
	}

	// Chmod +x
	if err := os.Chmod(hookPath, 0755); err != nil {
		ui.Fail("실행 권한 부여 실패: " + err.Error())
		return
	}
	ui.Success(fmt.Sprintf("'%s' hook 활성화 완료", name))
}

func doHookDisable(name string) {
	hooksDir := getHooksDir()
	hookPath := filepath.Join(hooksDir, name)

	if _, err := os.Stat(hookPath); os.IsNotExist(err) {
		ui.Warn(fmt.Sprintf("'%s' hook이 존재하지 않습니다", name))
		return
	}

	// Remove executable bit
	if err := os.Chmod(hookPath, 0644); err != nil {
		ui.Fail("권한 변경 실패: " + err.Error())
		return
	}
	ui.Success(fmt.Sprintf("'%s' hook 비활성화 완료 (실행권한 제거)", name))
	fmt.Println()
}

// Preset hook templates
var hookPresets = map[string]struct {
	desc    string
	content string
}{
	"commit-msg (Conventional Commits 검사)": {
		desc: "커밋 메시지가 feat:, fix:, ... 형식인지 검사합니다",
		content: `#!/bin/sh
# Validate Conventional Commits format
# Types: feat, fix, docs, style, refactor, perf, test, build, ci, chore, revert
COMMIT_MSG=$(cat "$1")
PATTERN='^(feat|fix|docs|style|refactor|perf|test|build|ci|chore|revert)(\([a-z0-9_-]+\))?!?: .+'
if ! echo "$COMMIT_MSG" | grep -Eq "$PATTERN"; then
  echo ""
  echo "  ✘ 커밋 메시지 형식 오류!"
  echo "  Conventional Commits 형식을 사용하세요:"
  echo "  예) feat: 로그인 기능 추가"
  echo "      fix(auth): 토큰 만료 버그 수정"
  echo ""
  exit 1
fi
`,
	},
	"pre-commit (공백·파일크기 검사)": {
		desc: "trailing whitespace, 5MB 초과 파일을 차단합니다",
		content: `#!/bin/sh
# Check for trailing whitespace and large files
ERRORS=0
# Trailing whitespace
git diff --cached --check
if [ $? -ne 0 ]; then
  echo "  ✘ trailing whitespace 발견 — 수정 후 다시 커밋하세요"
  ERRORS=1
fi
# Large files (> 5MB)
git diff --cached --name-only | while read FILE; do
  SIZE=$(git cat-file -s ":$FILE" 2>/dev/null || echo 0)
  if [ "$SIZE" -gt 5242880 ]; then
    echo "  ✘ 파일이 너무 큽니다 (5MB 초과): $FILE"
    ERRORS=1
  fi
done
exit $ERRORS
`,
	},
	"pre-push (테스트 실행)": {
		desc: "push 전에 go test ./... 를 실행합니다 (Go 프로젝트)",
		content: `#!/bin/sh
# Run tests before push
echo "  → 테스트 실행 중..."
go test ./...
if [ $? -ne 0 ]; then
  echo "  ✘ 테스트 실패 — push를 중단합니다"
  exit 1
fi
echo "  ✔ 테스트 통과"
`,
	},
}

func runHookInstall() {
	hooksDir := getHooksDir()
	if hooksDir == "" {
		ui.Fail("hooks 디렉토리를 찾을 수 없습니다")
		return
	}

	presetNames := make([]string, 0, len(hookPresets))
	for k := range hookPresets {
		presetNames = append(presetNames, k)
	}

	var selections []string
	if err := survey.AskOne(&survey.MultiSelect{
		Message: "설치할 프리셋 hook 선택:",
		Options: presetNames,
	}, &selections); err != nil || len(selections) == 0 {
		return
	}

	fmt.Println()
	for _, sel := range selections {
		preset := hookPresets[sel]
		// Determine hook file name from preset key
		hookName := strings.Fields(sel)[0]
		hookPath := filepath.Join(hooksDir, hookName)

		// Check if already exists
		if _, err := os.Stat(hookPath); err == nil {
			var overwrite bool
			_ = survey.AskOne(&survey.Confirm{
				Message: fmt.Sprintf("'%s' 이미 존재합니다. 덮어쓸까요?", hookName),
				Default: false,
			}, &overwrite)
			if !overwrite {
				continue
			}
		}

		if err := os.WriteFile(hookPath, []byte(preset.content), 0755); err != nil {
			ui.Fail(fmt.Sprintf("'%s' 설치 실패: %s", hookName, err.Error()))
			continue
		}
		ui.Success(fmt.Sprintf("'%s' 설치 완료 — %s", hookName, preset.desc))
	}
	fmt.Println()
}
