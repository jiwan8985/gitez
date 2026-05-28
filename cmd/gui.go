package cmd

import (
	"fmt"
	"gez/internal/webui"
	"os"
	"os/exec"
	"runtime"
	"time"

	"github.com/spf13/cobra"
)

var guiCmd = &cobra.Command{
	Use:     "gui",
	Short:   "브라우저 기반 Git GUI 실행",
	Long:    `웹 브라우저에서 Git GUI를 실행합니다. GitKraken/SourceTree처럼 클릭으로 git 작업을 수행할 수 있습니다.`,
	Aliases: []string{"web"},
	Run: func(cmd *cobra.Command, args []string) {
		port, _ := cmd.Flags().GetInt("port")
		dir, _ := os.Getwd()
		runGUI(dir, port)
	},
}

func runGUI(dir string, port int) {
	runGUIFull(dir, port, true)
}

func runGUIFull(dir string, port int, browser bool) {
	addr := fmt.Sprintf("http://localhost:%d", port)
	fmt.Printf("\n  gez GUI  →  %s\n\n", addr)
	fmt.Printf("  종료: Ctrl+C\n\n")

	if browser {
		go func() {
			time.Sleep(300 * time.Millisecond)
			openBrowser(addr)
		}()
	}

	srv := webui.NewServer(dir, port)
	if err := srv.ListenAndServe(); err != nil {
		fmt.Printf("서버 오류: %v\n", err)
		os.Exit(1)
	}
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", url)
	default:
		// Linux: try xdg-open, then fallback
		cmd = exec.Command("xdg-open", url)
	}
	_ = cmd.Start()
}

func init() {
	guiCmd.Flags().IntP("port", "P", 7777, "HTTP 포트")
	rootCmd.AddCommand(guiCmd)
}
