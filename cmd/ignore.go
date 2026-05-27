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

var ignoreCmd = &cobra.Command{
	Use:   "ignore",
	Short: ".gitignore 관리 (추가·목록·템플릿)",
	Run: func(cmd *cobra.Command, args []string) {
		if !git.IsRepo() {
			ui.Fail("git 저장소가 아닙니다")
			os.Exit(1)
		}
		runIgnoreMenu()
	},
}

var ignoreAddCmd = &cobra.Command{
	Use:   "add <패턴>",
	Short: ".gitignore에 패턴 추가",
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		for _, pattern := range args {
			doIgnoreAdd(pattern)
		}
	},
}

var ignoreListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   ".gitignore 내용 보기",
	Run: func(cmd *cobra.Command, args []string) { printIgnoreList() },
}

var ignoreCheckCmd = &cobra.Command{
	Use:   "check <경로>",
	Short: "파일이 gitignore 규칙에 걸리는지 확인",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		out, err := git.Run("check-ignore", "-v", args[0])
		fmt.Println()
		if err != nil {
			fmt.Printf("  %s  '%s' 은(는) gitignore 규칙에 해당하지 않습니다\n\n",
				ui.Green("✔"), args[0])
		} else {
			fmt.Printf("  %s  '%s' 은(는) 무시됩니다:\n", ui.Yellow("⚠"), args[0])
			fmt.Printf("    %s\n\n", ui.Dim(out))
		}
	},
}

func init() {
	ignoreCmd.AddCommand(ignoreAddCmd)
	ignoreCmd.AddCommand(ignoreListCmd)
	ignoreCmd.AddCommand(ignoreCheckCmd)
	rootCmd.AddCommand(ignoreCmd)
}

// ── Templates ─────────────────────────────────────────────────────────────────

var ignoreTemplates = map[string]string{
	"Go": `# Go
*.exe
*.exe~
*.dll
*.so
*.dylib
*.test
*.out
go.work
vendor/
`,
	"Node.js": `# Node.js
node_modules/
npm-debug.log*
yarn-debug.log*
yarn-error.log*
.pnpm-debug.log*
.env
.env.local
.env.*.local
dist/
build/
.cache/
`,
	"Python": `# Python
__pycache__/
*.py[cod]
*$py.class
*.so
.env
.venv
env/
venv/
*.egg-info/
dist/
build/
.pytest_cache/
.mypy_cache/
`,
	"Java": `# Java
*.class
*.jar
*.war
*.ear
target/
.gradle/
build/
*.iml
.idea/
*.iws
`,
	"Rust": `# Rust
/target/
Cargo.lock
**/*.rs.bk
`,
	"macOS": `# macOS
.DS_Store
.AppleDouble
.LSOverride
Icon
._*
.Spotlight-V100
.Trashes
`,
	"Windows": `# Windows
Thumbs.db
Thumbs.db:encryptable
ehthumbs.db
ehthumbs_vista.db
*.stackdump
[Dd]esktop.ini
$RECYCLE.BIN/
*.cab
*.msi
*.msix
*.msm
*.msp
*.lnk
`,
	"IDE (VS Code)": `# VS Code
.vscode/*
!.vscode/settings.json
!.vscode/tasks.json
!.vscode/launch.json
!.vscode/extensions.json
*.code-workspace
.history/
`,
	"IDE (JetBrains)": `# JetBrains
.idea/
*.iml
*.iws
*.ipr
out/
`,
	"Docker": `# Docker
.dockerignore
docker-compose.override.yml
`,
	"환경변수 파일": `# 환경변수
.env
.env.*
!.env.example
*.pem
*.key
`,
	"로그 & 임시파일": `# 로그 & 임시
*.log
logs/
*.tmp
*.temp
*.swp
*~
`,
}

// ── Interactive menu ───────────────────────────────────────────────────────────

func runIgnoreMenu() {
	printIgnoreList()

	var action string
	if err := survey.AskOne(&survey.Select{
		Message: "작업 선택:",
		Options: []string{
			"패턴 직접 추가",
			"템플릿에서 추가",
			"gitignore 파일 열기 (편집기)",
			"파일 무시 여부 확인",
			"취소",
		},
	}, &action); err != nil || action == "취소" {
		return
	}

	fmt.Println()
	switch {
	case strings.HasPrefix(action, "패턴 직접"):
		var pattern string
		if err := survey.AskOne(&survey.Input{
			Message: "추가할 패턴:",
			Help:    "예: *.log   build/   .env",
		}, &pattern, survey.WithValidator(survey.Required)); err != nil {
			return
		}
		doIgnoreAdd(strings.TrimSpace(pattern))

	case strings.HasPrefix(action, "템플릿"):
		names := make([]string, 0, len(ignoreTemplates))
		for k := range ignoreTemplates {
			names = append(names, k)
		}
		var selected []string
		if err := survey.AskOne(&survey.MultiSelect{
			Message: "추가할 템플릿 선택:",
			Options: names,
		}, &selected); err != nil || len(selected) == 0 {
			return
		}
		for _, name := range selected {
			content := ignoreTemplates[name]
			if err := appendToGitignore("\n" + content); err != nil {
				ui.Fail(fmt.Sprintf("'%s' 템플릿 추가 실패", name))
			} else {
				ui.Success(fmt.Sprintf("'%s' 템플릿 추가 완료", name))
			}
		}

	case strings.HasPrefix(action, "gitignore 파일 열기"):
		igPath := gitignorePath()
		editor, _ := git.Run("config", "--get", "core.editor")
		if editor == "" {
			editor = os.Getenv("EDITOR")
		}
		if editor == "" {
			editor = "notepad"
		}
		// Ensure file exists
		if _, err := os.Stat(igPath); os.IsNotExist(err) {
			os.WriteFile(igPath, []byte(""), 0644)
		}
		if err := git.RunLive(editor, igPath); err != nil {
			ui.Warn("편집기 실행 실패: 직접 열어주세요: " + igPath)
		}

	case strings.HasPrefix(action, "파일 무시 여부"):
		var path string
		if err := survey.AskOne(&survey.Input{
			Message: "확인할 파일 경로:",
		}, &path, survey.WithValidator(survey.Required)); err != nil {
			return
		}
		out, err := git.Run("check-ignore", "-v", strings.TrimSpace(path))
		if err != nil {
			fmt.Printf("  %s  해당 파일은 무시되지 않습니다\n\n", ui.Green("✔"))
		} else {
			fmt.Printf("  %s  무시 규칙: %s\n\n", ui.Yellow("⚠"), ui.Dim(out))
		}
	}
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func gitignorePath() string {
	root, err := git.Run("rev-parse", "--show-toplevel")
	if err != nil {
		return ".gitignore"
	}
	return filepath.Join(strings.TrimSpace(root), ".gitignore")
}

func printIgnoreList() {
	igPath := gitignorePath()
	data, err := os.ReadFile(igPath)
	fmt.Println()
	if err != nil {
		fmt.Printf("  %s\n\n", ui.Dim(".gitignore 파일이 없습니다"))
		return
	}
	lines := strings.Split(string(data), "\n")
	fmt.Printf("  %s  (%d줄)\n\n", ui.Bold(".gitignore"), len(lines))
	for _, l := range lines {
		if strings.TrimSpace(l) == "" {
			fmt.Println()
		} else if strings.HasPrefix(l, "#") {
			fmt.Printf("  %s\n", ui.Dim(l))
		} else {
			fmt.Printf("  %s\n", ui.Cyan(l))
		}
	}
	fmt.Println()
}

func doIgnoreAdd(pattern string) {
	igPath := gitignorePath()
	// Check if already exists
	if data, err := os.ReadFile(igPath); err == nil {
		for _, l := range strings.Split(string(data), "\n") {
			if strings.TrimSpace(l) == pattern {
				ui.Info(fmt.Sprintf("'%s' 는 이미 .gitignore에 있습니다", pattern))
				return
			}
		}
	}
	if err := appendToGitignore(pattern); err != nil {
		ui.Fail("추가 실패: " + err.Error())
		return
	}
	ui.Success(fmt.Sprintf("'%s' 추가 완료", pattern))
}

func appendToGitignore(content string) error {
	igPath := gitignorePath()
	f, err := os.OpenFile(igPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(content + "\n")
	return err
}
