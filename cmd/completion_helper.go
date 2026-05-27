package cmd

import (
	"fmt"
	"gez/internal/ui"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"
)

var completionHelperCmd = &cobra.Command{
	Use:   "completion-install",
	Short: "쉘 자동완성 설치 (bash·zsh·fish·PowerShell)",
	Run: func(cmd *cobra.Command, args []string) {
		runCompletionInstall()
	},
}

func init() {
	rootCmd.AddCommand(completionHelperCmd)
}

func runCompletionInstall() {
	fmt.Println()
	fmt.Printf("  %s\n\n", ui.Bold("gez 쉘 자동완성 설치"))

	// Detect current shell
	detectedShell := detectShell()
	fmt.Printf("  %s  %s\n\n", ui.Dim("감지된 쉘:"), ui.Cyan(detectedShell))

	var shell string
	if err := survey.AskOne(&survey.Select{
		Message: "자동완성을 설치할 쉘:",
		Options: []string{"bash", "zsh", "fish", "powershell"},
		Default: detectedShell,
	}, &shell); err != nil {
		return
	}

	fmt.Println()
	switch shell {
	case "bash":
		installBashCompletion()
	case "zsh":
		installZshCompletion()
	case "fish":
		installFishCompletion()
	case "powershell":
		installPowerShellCompletion()
	}
}

func detectShell() string {
	if runtime.GOOS == "windows" {
		return "powershell"
	}
	shell := os.Getenv("SHELL")
	switch {
	case strings.Contains(shell, "zsh"):
		return "zsh"
	case strings.Contains(shell, "fish"):
		return "fish"
	case strings.Contains(shell, "bash"):
		return "bash"
	default:
		return "bash"
	}
}

func installBashCompletion() {
	completionDir := "/etc/bash_completion.d"
	homeDir, _ := os.UserHomeDir()
	localDir := filepath.Join(homeDir, ".local", "share", "bash-completion", "completions")

	fmt.Printf("  %s\n\n", ui.Bold("Bash 자동완성 설치 방법:"))
	fmt.Printf("  %s  1. 완성 스크립트 생성:\n", ui.Dim("방법 A (시스템 전역):"))
	fmt.Printf("     %s\n\n", ui.Cyan("gez completion bash | sudo tee "+completionDir+"/gez"))

	fmt.Printf("  %s  1. 완성 스크립트 생성:\n", ui.Dim("방법 B (사용자):"))
	fmt.Printf("     %s\n", ui.Cyan("mkdir -p "+localDir))
	fmt.Printf("     %s\n\n", ui.Cyan("gez completion bash > "+filepath.Join(localDir, "gez")))

	fmt.Printf("  %s  2. ~/.bashrc에 추가:\n", ui.Dim(""))
	fmt.Printf("     %s\n\n", ui.Cyan(`echo 'source <(gez completion bash)' >> ~/.bashrc`))

	var auto bool
	_ = survey.AskOne(&survey.Confirm{
		Message: "~/.bashrc에 자동으로 추가할까요?",
		Default: true,
	}, &auto)
	if auto {
		appendToRCFile(filepath.Join(homeDir, ".bashrc"),
			"\n# gez shell completion\nsource <(gez completion bash)\n")
	}
}

func installZshCompletion() {
	homeDir, _ := os.UserHomeDir()

	fmt.Printf("  %s\n\n", ui.Bold("Zsh 자동완성 설치 방법:"))
	fmt.Printf("  %s  ~/.zshrc에 추가:\n\n", ui.Dim("방법 A:"))
	fmt.Printf("     %s\n\n", ui.Cyan(`echo 'source <(gez completion zsh)' >> ~/.zshrc`))

	fmt.Printf("  %s  Oh My Zsh 사용 시:\n", ui.Dim("방법 B:"))
	zshDir := filepath.Join(homeDir, ".oh-my-zsh", "completions")
	fmt.Printf("     %s\n", ui.Cyan("mkdir -p "+zshDir))
	fmt.Printf("     %s\n\n", ui.Cyan("gez completion zsh > "+filepath.Join(zshDir, "_gez")))

	var auto bool
	_ = survey.AskOne(&survey.Confirm{
		Message: "~/.zshrc에 자동으로 추가할까요?",
		Default: true,
	}, &auto)
	if auto {
		appendToRCFile(filepath.Join(homeDir, ".zshrc"),
			"\n# gez shell completion\nsource <(gez completion zsh)\n")
	}
}

func installFishCompletion() {
	homeDir, _ := os.UserHomeDir()
	fishDir := filepath.Join(homeDir, ".config", "fish", "completions")

	fmt.Printf("  %s\n\n", ui.Bold("Fish 자동완성 설치 방법:"))
	fmt.Printf("     %s\n", ui.Cyan("mkdir -p "+fishDir))
	fmt.Printf("     %s\n\n", ui.Cyan("gez completion fish > "+filepath.Join(fishDir, "gez.fish")))

	var auto bool
	_ = survey.AskOne(&survey.Confirm{
		Message: "자동으로 설치할까요?",
		Default: true,
	}, &auto)
	if auto {
		os.MkdirAll(fishDir, 0755)
		outFile := filepath.Join(fishDir, "gez.fish")
		if f, err := os.Create(outFile); err == nil {
			rootCmd.GenFishCompletion(f, true)
			f.Close()
			ui.Success("Fish 자동완성 설치 완료: " + outFile)
		} else {
			ui.Fail("설치 실패: " + err.Error())
		}
	}
}

func installPowerShellCompletion() {
	homeDir, _ := os.UserHomeDir()
	profilePath := filepath.Join(homeDir, "Documents", "PowerShell", "Microsoft.PowerShell_profile.ps1")

	fmt.Printf("  %s\n\n", ui.Bold("PowerShell 자동완성 설치 방법:"))
	fmt.Printf("  %s  1. 완성 스크립트 생성:\n", ui.Dim(""))
	fmt.Printf("     %s\n\n", ui.Cyan("gez completion powershell >> $PROFILE"))

	fmt.Printf("  %s  2. 또는 profile.ps1에 직접 추가:\n     %s\n\n",
		ui.Dim(""),
		ui.Cyan(`gez completion powershell | Out-String | Invoke-Expression`))

	var auto bool
	_ = survey.AskOne(&survey.Confirm{
		Message: "PowerShell 프로필에 자동으로 추가할까요?",
		Default: true,
	}, &auto)
	if auto {
		os.MkdirAll(filepath.Dir(profilePath), 0755)
		appendToRCFile(profilePath,
			"\n# gez shell completion\ngez completion powershell | Out-String | Invoke-Expression\n")
	}
}

func appendToRCFile(path, content string) {
	// Ensure parent directory exists
	os.MkdirAll(filepath.Dir(path), 0755)

	// Check if already added
	if data, err := os.ReadFile(path); err == nil {
		if strings.Contains(string(data), "gez completion") {
			ui.Info(fmt.Sprintf("이미 %s에 추가되어 있습니다", filepath.Base(path)))
			return
		}
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		ui.Fail("파일 쓰기 실패: " + err.Error())
		return
	}
	defer f.Close()
	if _, err := f.WriteString(content); err != nil {
		ui.Fail("쓰기 실패: " + err.Error())
		return
	}
	ui.Success(fmt.Sprintf("자동완성 추가 완료! → %s", path))
	fmt.Printf("  %s  쉘을 재시작하거나 %s 을 실행하세요\n\n",
		ui.Dim("Tip:"), ui.Cyan("source "+path))
}
