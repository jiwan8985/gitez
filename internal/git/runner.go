package git

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Run executes a git command in the current directory and returns stdout (trimmed).
func Run(args ...string) (string, error) {
	c := exec.Command("git", args...)
	var stdout, stderr bytes.Buffer
	c.Stdout = &stdout
	c.Stderr = &stderr
	if err := c.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("%s", msg)
	}
	return strings.TrimSpace(stdout.String()), nil
}

// RunLive executes a git command with output piped directly to the terminal.
func RunLive(args ...string) error {
	c := exec.Command("git", args...)
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	c.Stdin = os.Stdin
	return c.Run()
}

// RunInDir executes a git command in the specified directory (uses git -C).
func RunInDir(dir string, args ...string) (string, error) {
	all := append([]string{"-C", dir}, args...)
	return Run(all...)
}

// RunLiveInDir executes a git command in the specified directory with live output.
func RunLiveInDir(dir string, args ...string) error {
	all := append([]string{"-C", dir}, args...)
	return RunLive(all...)
}

// ── Current-directory helpers ─────────────────────────────────────────────────

// IsRepo checks if the current directory is inside a git repository.
func IsRepo() bool {
	_, err := Run("rev-parse", "--is-inside-work-tree")
	return err == nil
}

// CurrentBranch returns the active branch name.
func CurrentBranch() string {
	b, _ := Run("branch", "--show-current")
	return b
}

// LocalBranches returns all local branch names.
func LocalBranches() []string {
	out, err := Run("branch", "--format=%(refname:short)")
	if err != nil || out == "" {
		return nil
	}
	return strings.Split(out, "\n")
}

// RemoteBranches returns remote-tracking branch names (excludes HEAD).
func RemoteBranches() []string {
	out, err := Run("branch", "-r", "--format=%(refname:short)")
	if err != nil || out == "" {
		return nil
	}
	var result []string
	for _, l := range strings.Split(out, "\n") {
		l = strings.TrimSpace(l)
		if l != "" && !strings.Contains(l, "HEAD") {
			result = append(result, l)
		}
	}
	return result
}

// StatusShort returns lines from `git status --short`.
func StatusShort() []string {
	out, err := Run("status", "--short")
	if err != nil || out == "" {
		return nil
	}
	return strings.Split(out, "\n")
}

// UnstagedFiles returns files with unstaged changes (XY where Y != ' ').
func UnstagedFiles() []string {
	lines := StatusShort()
	var files []string
	for _, l := range lines {
		if len(l) < 3 {
			continue
		}
		y := l[1]
		if y != ' ' {
			files = append(files, strings.TrimSpace(l[3:]))
		}
	}
	return files
}

// AheadBehind returns (ahead, behind) commit counts relative to upstream.
func AheadBehind() (string, string) {
	out, err := Run("rev-list", "--left-right", "--count", "HEAD...@{upstream}")
	if err != nil {
		return "", ""
	}
	parts := strings.Fields(out)
	if len(parts) != 2 {
		return "", ""
	}
	return parts[0], parts[1]
}

// Remotes returns a list of configured remote names.
func Remotes() []string {
	out, err := Run("remote")
	if err != nil || out == "" {
		return nil
	}
	return strings.Split(out, "\n")
}

// Tags returns a list of tag names.
func Tags() []string {
	out, err := Run("tag", "--sort=-version:refname")
	if err != nil || out == "" {
		return nil
	}
	return strings.Split(out, "\n")
}

// StashList returns stash entries as "index: message".
func StashList() []string {
	out, err := Run("stash", "list")
	if err != nil || out == "" {
		return nil
	}
	return strings.Split(out, "\n")
}

// ── Operation-state helpers ───────────────────────────────────────────────────

// gitDir returns the path to the .git directory (works with worktrees too).
func gitDir() string {
	out, err := Run("rev-parse", "--git-dir")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(out)
}

// IsRebaseInProgress reports whether a rebase is currently in progress.
func IsRebaseInProgress() bool {
	d := gitDir()
	if d == "" {
		return false
	}
	_, err1 := os.Stat(filepath.Join(d, "rebase-merge"))
	_, err2 := os.Stat(filepath.Join(d, "rebase-apply"))
	return err1 == nil || err2 == nil
}

// IsCherryPickInProgress reports whether a cherry-pick is in progress.
func IsCherryPickInProgress() bool {
	d := gitDir()
	if d == "" {
		return false
	}
	_, err := os.Stat(filepath.Join(d, "CHERRY_PICK_HEAD"))
	return err == nil
}

// IsMergeInProgress reports whether a merge is in progress.
func IsMergeInProgress() bool {
	d := gitDir()
	if d == "" {
		return false
	}
	_, err := os.Stat(filepath.Join(d, "MERGE_HEAD"))
	return err == nil
}

// IsRevertInProgress reports whether a revert is in progress.
func IsRevertInProgress() bool {
	d := gitDir()
	if d == "" {
		return false
	}
	_, err := os.Stat(filepath.Join(d, "REVERT_HEAD"))
	return err == nil
}

// RecentCommits returns the last n commits as "hash  subject" one-liners.
func RecentCommits(n int) []string {
	out, err := Run("log", "--oneline", fmt.Sprintf("-n%d", n))
	if err != nil || out == "" {
		return nil
	}
	var result []string
	for _, l := range strings.Split(out, "\n") {
		if l != "" {
			result = append(result, l)
		}
	}
	return result
}

// CommitsBetween returns commits in the from..to range as one-liners.
func CommitsBetween(from, to string) []string {
	out, err := Run("log", "--oneline", fmt.Sprintf("%s..%s", from, to))
	if err != nil || out == "" {
		return nil
	}
	var result []string
	for _, l := range strings.Split(out, "\n") {
		if l != "" {
			result = append(result, l)
		}
	}
	return result
}

// ReflogEntries returns reflog entries as one-liners (newest first).
func ReflogEntries(n int) []string {
	out, err := Run("reflog", "--oneline", fmt.Sprintf("-n%d", n))
	if err != nil || out == "" {
		return nil
	}
	var result []string
	for _, l := range strings.Split(out, "\n") {
		if l != "" {
			result = append(result, l)
		}
	}
	return result
}

// TrackedFiles returns all files tracked by git in the current repository.
func TrackedFiles() []string {
	out, err := Run("ls-files")
	if err != nil || out == "" {
		return nil
	}
	var result []string
	for _, l := range strings.Split(out, "\n") {
		if l != "" {
			result = append(result, l)
		}
	}
	return result
}

// UntrackedFiles returns untracked files. Set withIgnored=true to include .gitignore'd files.
func UntrackedFiles(withIgnored bool) []string {
	args := []string{"ls-files", "--others", "--exclude-standard"}
	if withIgnored {
		args = append(args, "-i")
	}
	out, err := Run(args...)
	if err != nil || out == "" {
		return nil
	}
	var result []string
	for _, l := range strings.Split(out, "\n") {
		if l != "" {
			result = append(result, l)
		}
	}
	return result
}

// ── Multi-directory helpers (used by workspace commands) ──────────────────────

// IsRepoInDir checks if the specified directory is inside a git repository.
func IsRepoInDir(dir string) bool {
	_, err := RunInDir(dir, "rev-parse", "--is-inside-work-tree")
	return err == nil
}

// CurrentBranchInDir returns the active branch name in the specified directory.
func CurrentBranchInDir(dir string) string {
	b, _ := RunInDir(dir, "branch", "--show-current")
	return b
}

// StatusShortInDir returns lines from `git status --short` in the specified directory.
func StatusShortInDir(dir string) []string {
	out, err := RunInDir(dir, "status", "--short")
	if err != nil || out == "" {
		return nil
	}
	return strings.Split(out, "\n")
}

// AheadBehindInDir returns (ahead, behind) commit counts for the specified directory.
func AheadBehindInDir(dir string) (string, string) {
	out, err := RunInDir(dir, "rev-list", "--left-right", "--count", "HEAD...@{upstream}")
	if err != nil {
		return "", ""
	}
	parts := strings.Fields(out)
	if len(parts) != 2 {
		return "", ""
	}
	return parts[0], parts[1]
}
