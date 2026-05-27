package cmd

import (
	"fmt"
	"gez/internal/git"
	"gez/internal/ui"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

var statsCmd = &cobra.Command{
	Use:   "stats",
	Short: "저장소 통계 (기여자·커밋 수·파일·활동)",
	Run: func(cmd *cobra.Command, args []string) {
		if !git.IsRepo() {
			ui.Fail("git 저장소가 아닙니다")
			os.Exit(1)
		}
		runStats()
	},
}

func init() {
	rootCmd.AddCommand(statsCmd)
}

func runStats() {
	sep := ui.Dim(strings.Repeat("─", 56))
	fmt.Println()
	fmt.Printf("  %s\n\n", ui.Bold("저장소 통계"))

	// ── 기본 정보 ───────────────────────────────────────────────
	fmt.Println(sep)
	fmt.Printf("  %s\n\n", ui.Bold("기본 정보"))

	branch := git.CurrentBranch()
	totalCommits, _ := git.Run("rev-list", "--count", "HEAD")
	firstCommit, _ := git.Run("log", "--reverse", "--format=%ad  (%ar)", "--date=short", "-1")
	lastCommit, _ := git.Run("log", "--format=%ad  (%ar)", "--date=short", "-1")
	repoRoot, _ := git.Run("rev-parse", "--show-toplevel")

	fmt.Printf("  %-18s  %s\n", "현재 브랜치:", ui.BoldCyan(branch))
	fmt.Printf("  %-18s  %s\n", "총 커밋 수:", ui.Yellow(strings.TrimSpace(totalCommits)))
	fmt.Printf("  %-18s  %s\n", "첫 커밋:", ui.Dim(strings.TrimSpace(firstCommit)))
	fmt.Printf("  %-18s  %s\n", "최근 커밋:", ui.Dim(strings.TrimSpace(lastCommit)))
	fmt.Printf("  %-18s  %s\n", "저장소 경로:", ui.Dim(strings.TrimSpace(repoRoot)))
	fmt.Println()

	// ── 기여자 ──────────────────────────────────────────────────
	fmt.Println(sep)
	fmt.Printf("  %s\n\n", ui.Bold("기여자 (커밋 수 순)"))

	shortlog, _ := git.Run("shortlog", "-sn", "--all", "--no-merges")
	if shortlog != "" {
		lines := strings.Split(strings.TrimSpace(shortlog), "\n")
		maxShow := 10
		if len(lines) < maxShow {
			maxShow = len(lines)
		}
		for i, l := range lines[:maxShow] {
			parts := strings.Fields(l)
			if len(parts) < 2 {
				continue
			}
			count := parts[0]
			name := strings.Join(parts[1:], " ")
			bar := makeBar(count, lines[0])
			marker := ui.Dim(fmt.Sprintf("%2d.", i+1))
			fmt.Printf("  %s  %-24s  %s  %s\n",
				marker, name, ui.Green(bar), ui.Yellow(count))
		}
		if len(lines) > maxShow {
			fmt.Printf("  %s\n", ui.Dim(fmt.Sprintf("    ... 외 %d명", len(lines)-maxShow)))
		}
	}
	fmt.Println()

	// ── 파일 통계 ───────────────────────────────────────────────
	fmt.Println(sep)
	fmt.Printf("  %s\n\n", ui.Bold("파일 통계"))

	trackedFiles := git.TrackedFiles()
	extCount := make(map[string]int)
	for _, f := range trackedFiles {
		ext := fileExt(f)
		extCount[ext]++
	}
	type extEntry struct {
		ext   string
		count int
	}
	var exts []extEntry
	for k, v := range extCount {
		exts = append(exts, extEntry{k, v})
	}
	sort.Slice(exts, func(i, j int) bool { return exts[i].count > exts[j].count })

	fmt.Printf("  %-18s  %s\n", "총 파일 수:", ui.Yellow(strconv.Itoa(len(trackedFiles))))
	fmt.Println()
	fmt.Printf("  %s\n", ui.Dim("확장자별 파일 수:"))
	maxShow := 10
	if len(exts) < maxShow {
		maxShow = len(exts)
	}
	for _, e := range exts[:maxShow] {
		fmt.Printf("    %-12s  %s\n", ui.Cyan(e.ext), ui.Yellow(strconv.Itoa(e.count)))
	}
	fmt.Println()

	// ── 가장 많이 변경된 파일 ───────────────────────────────────
	fmt.Println(sep)
	fmt.Printf("  %s\n\n", ui.Bold("가장 많이 변경된 파일 (상위 10)"))

	numstatOut, _ := git.Run("log", "--pretty=format:", "--name-only", "--no-merges")
	if numstatOut != "" {
		fileFreq := make(map[string]int)
		for _, line := range strings.Split(numstatOut, "\n") {
			line = strings.TrimSpace(line)
			if line != "" {
				fileFreq[line]++
			}
		}
		type freq struct {
			file  string
			count int
		}
		var freqs []freq
		for f, c := range fileFreq {
			freqs = append(freqs, freq{f, c})
		}
		sort.Slice(freqs, func(i, j int) bool { return freqs[i].count > freqs[j].count })
		lim := 10
		if len(freqs) < lim {
			lim = len(freqs)
		}
		for i, f := range freqs[:lim] {
			fmt.Printf("  %s  %-40s  %s 번\n",
				ui.Dim(fmt.Sprintf("%2d.", i+1)),
				truncate(f.file, 40),
				ui.Yellow(strconv.Itoa(f.count)))
		}
	}
	fmt.Println()

	// ── 월별 커밋 활동 ──────────────────────────────────────────
	fmt.Println(sep)
	fmt.Printf("  %s\n\n", ui.Bold("최근 12개월 커밋 활동"))

	monthOut, _ := git.Run("log", "--format=%ad", "--date=format:%Y-%m", "--no-merges")
	if monthOut != "" {
		monthCount := make(map[string]int)
		for _, m := range strings.Split(monthOut, "\n") {
			m = strings.TrimSpace(m)
			if m != "" {
				monthCount[m]++
			}
		}
		// Get sorted months (last 12)
		var months []string
		for m := range monthCount {
			months = append(months, m)
		}
		sort.Sort(sort.Reverse(sort.StringSlice(months)))
		if len(months) > 12 {
			months = months[:12]
		}
		sort.Strings(months)
		maxVal := 0
		for _, m := range months {
			if monthCount[m] > maxVal {
				maxVal = monthCount[m]
			}
		}
		for _, m := range months {
			c := monthCount[m]
			bar := makeBarN(c, maxVal, 20)
			fmt.Printf("  %s  %s  %s\n", ui.Dim(m), ui.Green(bar), ui.Yellow(strconv.Itoa(c)))
		}
	}
	fmt.Println(sep)
	fmt.Println()
}

func makeBar(countStr string, maxStr string) string {
	count, _ := strconv.Atoi(strings.TrimSpace(countStr))
	maxVal, _ := strconv.Atoi(strings.TrimSpace(strings.Fields(maxStr)[0]))
	return makeBarN(count, maxVal, 12)
}

func makeBarN(count, maxVal, width int) string {
	if maxVal == 0 {
		return ""
	}
	n := count * width / maxVal
	if n < 1 && count > 0 {
		n = 1
	}
	return strings.Repeat("█", n) + strings.Repeat("░", width-n)
}

func fileExt(path string) string {
	parts := strings.Split(path, ".")
	if len(parts) < 2 {
		return "(no ext)"
	}
	return "." + parts[len(parts)-1]
}
