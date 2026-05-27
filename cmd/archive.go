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

var archiveCmd = &cobra.Command{
	Use:   "archive",
	Short: "저장소를 zip / tar.gz 파일로 내보내기",
	Run: func(cmd *cobra.Command, args []string) {
		if !git.IsRepo() {
			ui.Fail("git 저장소가 아닙니다")
			os.Exit(1)
		}
		runArchive()
	},
}

func init() {
	rootCmd.AddCommand(archiveCmd)
}

func runArchive() {
	fmt.Println()

	// Choose ref
	tags := git.Tags()
	refOptions := []string{"HEAD (현재 상태)"}
	refOptions = append(refOptions, tags...)
	refOptions = append(refOptions, "브랜치 선택", "직접 입력")

	var refSel string
	if err := survey.AskOne(&survey.Select{
		Message: "내보낼 ref (태그·브랜치·커밋):",
		Options: refOptions,
	}, &refSel); err != nil {
		return
	}

	var ref string
	switch {
	case refSel == "HEAD (현재 상태)":
		ref = "HEAD"
	case refSel == "브랜치 선택":
		branches := git.LocalBranches()
		if err := survey.AskOne(&survey.Select{
			Message: "브랜치 선택:",
			Options: branches,
		}, &ref); err != nil {
			return
		}
	case refSel == "직접 입력":
		if err := survey.AskOne(&survey.Input{
			Message: "ref (해시·태그·브랜치):",
		}, &ref, survey.WithValidator(survey.Required)); err != nil {
			return
		}
		ref = strings.TrimSpace(ref)
	default:
		ref = refSel
	}

	// Choose format
	var format string
	if err := survey.AskOne(&survey.Select{
		Message: "파일 형식:",
		Options: []string{"zip", "tar.gz", "tar"},
	}, &format); err != nil {
		return
	}

	// Choose output filename
	repoName := ""
	if root, err := git.Run("rev-parse", "--show-toplevel"); err == nil {
		parts := strings.Split(strings.TrimSpace(root), "/")
		if len(parts) > 0 {
			repoName = parts[len(parts)-1]
		}
	}
	if repoName == "" {
		repoName = "repo"
	}
	date := time.Now().Format("20060102")
	refClean := strings.ReplaceAll(ref, "/", "-")
	defaultName := fmt.Sprintf("%s-%s-%s.%s", repoName, refClean, date, format)

	var outFile string
	if err := survey.AskOne(&survey.Input{
		Message: "출력 파일명:",
		Default: defaultName,
	}, &outFile); err != nil {
		return
	}
	outFile = strings.TrimSpace(outFile)

	fmt.Println()
	ui.Info(fmt.Sprintf("아카이브 생성 중: %s  [%s]", outFile, ref))

	var err error
	switch format {
	case "zip":
		err = git.RunLive("archive", "--format=zip", "--output="+outFile, ref)
	case "tar.gz":
		err = git.RunLive("archive", "--format=tar.gz", "--output="+outFile, ref)
	default:
		err = git.RunLive("archive", "--format=tar", "--output="+outFile, ref)
	}

	fmt.Println()
	if err != nil {
		ui.Fail("아카이브 생성 실패: " + err.Error())
		return
	}

	// Get file size
	if fi, statErr := os.Stat(outFile); statErr == nil {
		size := fi.Size()
		sizeStr := fmt.Sprintf("%d KB", size/1024)
		if size >= 1024*1024 {
			sizeStr = fmt.Sprintf("%.1f MB", float64(size)/1024/1024)
		}
		ui.Success(fmt.Sprintf("아카이브 생성 완료! %s  (%s)", outFile, sizeStr))
	} else {
		ui.Success(fmt.Sprintf("아카이브 생성 완료! %s", outFile))
	}
	fmt.Println()
}
