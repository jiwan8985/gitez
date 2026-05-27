package cmd

import (
	"fmt"
	"gez/internal/git"
	"gez/internal/ui"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Git 환경 진단 (설정·버전·인증·연결 상태 확인)",
	Run: func(cmd *cobra.Command, args []string) {
		runDoctor()
	},
}

func init() {
	rootCmd.AddCommand(doctorCmd)
}

type checkResult struct {
	name   string
	ok     bool
	detail string
	fix    string
}

func runDoctor() {
	fmt.Println()
	fmt.Printf("  %s\n\n", ui.Bold("gez doctor — Git 환경 진단"))

	var results []checkResult

	// ── Git 버전 ─────────────────────────────────────────────────
	ver, err := git.Run("--version")
	if err != nil {
		results = append(results, checkResult{
			name:   "git 설치",
			ok:     false,
			detail: "git을 찾을 수 없습니다",
			fix:    "https://git-scm.com/downloads 에서 git을 설치하세요",
		})
	} else {
		results = append(results, checkResult{
			name:   "git 설치",
			ok:     true,
			detail: strings.TrimSpace(ver),
		})
	}

	// ── user.name ────────────────────────────────────────────────
	name, _ := git.Run("config", "--global", "user.name")
	if strings.TrimSpace(name) == "" {
		results = append(results, checkResult{
			name:   "user.name",
			ok:     false,
			detail: "설정되지 않음",
			fix:    `git config --global user.name "이름"`,
		})
	} else {
		results = append(results, checkResult{
			name:   "user.name",
			ok:     true,
			detail: strings.TrimSpace(name),
		})
	}

	// ── user.email ───────────────────────────────────────────────
	email, _ := git.Run("config", "--global", "user.email")
	if strings.TrimSpace(email) == "" {
		results = append(results, checkResult{
			name:   "user.email",
			ok:     false,
			detail: "설정되지 않음",
			fix:    `git config --global user.email "email@example.com"`,
		})
	} else {
		results = append(results, checkResult{
			name:   "user.email",
			ok:     true,
			detail: strings.TrimSpace(email),
		})
	}

	// ── core.editor ──────────────────────────────────────────────
	editor, _ := git.Run("config", "--global", "core.editor")
	if strings.TrimSpace(editor) == "" {
		results = append(results, checkResult{
			name:   "core.editor",
			ok:     false,
			detail: "설정되지 않음 (기본값 사용)",
			fix:    `git config --global core.editor "code --wait"`,
		})
	} else {
		results = append(results, checkResult{
			name:   "core.editor",
			ok:     true,
			detail: strings.TrimSpace(editor),
		})
	}

	// ── init.defaultBranch ───────────────────────────────────────
	defBranch, _ := git.Run("config", "--global", "init.defaultBranch")
	if strings.TrimSpace(defBranch) == "" {
		results = append(results, checkResult{
			name:   "init.defaultBranch",
			ok:     false,
			detail: "설정되지 않음 (git 기본값 사용)",
			fix:    `git config --global init.defaultBranch main`,
		})
	} else {
		results = append(results, checkResult{
			name:   "init.defaultBranch",
			ok:     true,
			detail: strings.TrimSpace(defBranch),
		})
	}

	// ── SSH key ──────────────────────────────────────────────────
	sshOk, sshDetail := checkSSHKeys()
	results = append(results, checkResult{
		name:   "SSH 키",
		ok:     sshOk,
		detail: sshDetail,
		fix:    `ssh-keygen -t ed25519 -C "email" 로 키 생성 후 GitHub에 등록`,
	})

	// ── Remote connectivity (if in git repo) ────────────────────
	if git.IsRepo() {
		remotes := git.Remotes()
		if len(remotes) > 0 {
			remote := remotes[0]
			_, connErr := git.Run("ls-remote", "--heads", remote)
			if connErr != nil {
				results = append(results, checkResult{
					name:   "remote 연결 (" + remote + ")",
					ok:     false,
					detail: "연결 실패",
					fix:    "네트워크 상태 및 인증 정보를 확인하세요",
				})
			} else {
				results = append(results, checkResult{
					name:   "remote 연결 (" + remote + ")",
					ok:     true,
					detail: "연결 OK",
				})
			}
		}

		// ── .gitignore ─────────────────────────────────────────
		igPath := gitignorePath()
		if _, statErr := os.Stat(igPath); statErr != nil {
			results = append(results, checkResult{
				name:   ".gitignore",
				ok:     false,
				detail: "파일 없음",
				fix:    "gez ignore 로 .gitignore를 생성하세요",
			})
		} else {
			results = append(results, checkResult{
				name:   ".gitignore",
				ok:     true,
				detail: "있음",
			})
		}
	}

	// ── OS / Platform ────────────────────────────────────────────
	results = append(results, checkResult{
		name:   "플랫폼",
		ok:     true,
		detail: fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH),
	})

	// ── 결과 출력 ────────────────────────────────────────────────
	sep := ui.Dim(strings.Repeat("─", 64))
	fmt.Println(sep)

	issues := 0
	for _, r := range results {
		icon := ui.Green("✔")
		nameCol := ui.Cyan(fmt.Sprintf("%-26s", r.name))
		if !r.ok {
			icon = ui.Red("✘")
			nameCol = ui.BoldRed(fmt.Sprintf("%-26s", r.name))
			issues++
		}
		fmt.Printf("  %s  %s  %s\n", icon, nameCol, ui.Dim(r.detail))
		if !r.ok && r.fix != "" {
			fmt.Printf("         %s  %s\n", ui.Dim("해결:"), ui.Yellow(r.fix))
		}
	}

	fmt.Println(sep)
	fmt.Println()
	if issues == 0 {
		ui.Success("모든 검사를 통과했습니다! 환경이 정상입니다.")
	} else {
		ui.Warn(fmt.Sprintf("%d개 항목에 문제가 있습니다. 위 해결 방법을 참고하세요.", issues))
	}
	fmt.Println()
}

func checkSSHKeys() (bool, string) {
	c := exec.Command("ssh-add", "-l")
	out, err := c.Output()
	if err != nil {
		return false, "ssh-agent 사용 불가 (HTTPS 인증 사용 중일 수 있음)"
	}
	outStr := strings.TrimSpace(string(out))
	if outStr == "The agent has no identities." {
		return false, "ssh-agent에 키 없음"
	}
	lines := strings.Split(outStr, "\n")
	return true, fmt.Sprintf("%d개 키 로드됨", len(lines))
}
