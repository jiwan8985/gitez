package cmd

import (
	"fmt"
	"gez/internal/tui"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

var uiCmd = &cobra.Command{
	Use:     "ui",
	Aliases: []string{"tui"},
	Short:   "전체화면 TUI 모드 (파일 스테이징·diff·로그 한눈에)",
	Run: func(cmd *cobra.Command, args []string) {
		m := tui.New()
		p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
		if _, err := p.Run(); err != nil {
			fmt.Printf("TUI 오류: %v\n", err)
		}
	},
}

func init() {
	rootCmd.AddCommand(uiCmd)
}
