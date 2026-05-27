package cmd

import (
	"fmt"
	"gez/internal/git"
	"gez/internal/ui"
	"os"
	"regexp"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"
)

var prBase string

var prCmd = &cobra.Command{
	Use:   "pr",
	Short: "Pull Request / Merge Request URL을 브라우저로 열기",
	Run: func(cmd *cobra.Command, args []string) {
		if !git.IsRepo() {
			ui.Fail("git 저장소가 아닙니다")
			os.Exit(1)
		}
		runPR()
	},
}

func init() {
	prCmd.Flags().StringVarP(&prBase, "base", "b", "", "대상 브랜치 (기본: main 또는 master)")
	rootCmd.AddCommand(prCmd)
}

func runPR() {
	// Get current branch
	current := git.CurrentBranch()
	if current == "" {
		ui.Fail("브랜치 정보를 가져올 수 없습니다")
		return
	}

	// Get remote URL
	remoteURL, err := git.Run("remote", "get-url", "origin")
	if err != nil {
		ui.Fail("origin remote를 찾을 수 없습니다")
		return
	}
	remoteURL = strings.TrimSpace(remoteURL)

	// Parse host and repo path
	host, repoPath := parseRemoteURL(remoteURL)
	if host == "" || repoPath == "" {
		ui.Fail("원격 URL 파싱 실패: " + remoteURL)
		return
	}

	// Determine base branch
	base := prBase
	if base == "" {
		base = detectDefaultBranch()
	}

	// Show info
	fmt.Println()
	fmt.Printf("  %s  %s\n", ui.Bold("저장소:"), ui.Cyan(repoPath))
	fmt.Printf("  %s  %s  →  %s\n",
		ui.Bold("브랜치:"),
		ui.BoldCyan(current),
		ui.Cyan(base))
	fmt.Println()

	// Confirm base
	var ok bool
	_ = survey.AskOne(&survey.Confirm{
		Message: fmt.Sprintf("'%s' → '%s' 로 PR을 열까요?", current, base),
		Default: true,
	}, &ok)
	if !ok {
		return
	}

	// Build URL
	url := buildPRURL(host, repoPath, current, base)
	if url == "" {
		ui.Warn(fmt.Sprintf("지원하지 않는 호스트입니다: %s", host))
		fmt.Printf("  %s  %s\n\n", ui.Dim("수동으로 PR 생성:"), remoteURL)
		return
	}

	fmt.Printf("  %s  %s\n\n", ui.Dim("URL:"), ui.Cyan(url))

	if err := git.OpenBrowser(url); err != nil {
		ui.Warn("브라우저 열기 실패. 위 URL을 직접 복사하세요")
	} else {
		ui.Success("브라우저에서 PR 페이지를 열었습니다!")
	}
	fmt.Println()
}

// parseRemoteURL extracts host and owner/repo from a git remote URL.
// Supports: https://github.com/user/repo.git  or  git@github.com:user/repo.git
func parseRemoteURL(rawURL string) (host, repoPath string) {
	rawURL = strings.TrimSuffix(rawURL, ".git")
	rawURL = strings.TrimSuffix(rawURL, "/")

	// HTTPS: https://github.com/user/repo
	httpsRe := regexp.MustCompile(`^https?://([^/]+)/(.+)$`)
	if m := httpsRe.FindStringSubmatch(rawURL); len(m) == 3 {
		return m[1], m[2]
	}

	// SSH: git@github.com:user/repo
	sshRe := regexp.MustCompile(`^git@([^:]+):(.+)$`)
	if m := sshRe.FindStringSubmatch(rawURL); len(m) == 3 {
		return m[1], m[2]
	}

	return "", ""
}

// buildPRURL constructs the PR creation URL for known hosting services.
func buildPRURL(host, repoPath, head, base string) string {
	switch {
	case strings.Contains(host, "github.com"):
		return fmt.Sprintf("https://github.com/%s/compare/%s...%s?expand=1",
			repoPath, base, head)

	case strings.Contains(host, "gitlab.com") || strings.Contains(host, "gitlab."):
		return fmt.Sprintf("https://%s/%s/-/merge_requests/new?merge_request%%5Bsource_branch%%5D=%s&merge_request%%5Btarget_branch%%5D=%s",
			host, repoPath, head, base)

	case strings.Contains(host, "bitbucket.org"):
		return fmt.Sprintf("https://bitbucket.org/%s/pull-requests/new?source=%s&dest=%s",
			repoPath, head, base)

	default:
		// Try GitHub enterprise pattern
		if !strings.Contains(host, "github.") {
			return ""
		}
		return fmt.Sprintf("https://%s/%s/compare/%s...%s?expand=1",
			host, repoPath, base, head)
	}
}

// detectDefaultBranch tries to determine the repo's default branch.
func detectDefaultBranch() string {
	// Check HEAD of origin
	symref, err := git.Run("remote", "show", "origin")
	if err == nil {
		for _, line := range strings.Split(symref, "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "HEAD branch:") {
				b := strings.TrimSpace(strings.TrimPrefix(line, "HEAD branch:"))
				if b != "" && b != "(unknown)" {
					return b
				}
			}
		}
	}

	// Fallback: check if main/master exist
	locals := git.LocalBranches()
	for _, b := range locals {
		if b == "main" {
			return "main"
		}
	}
	for _, b := range locals {
		if b == "master" {
			return "master"
		}
	}
	return "main"
}
