package cmd

import (
	"encoding/json"
	"fmt"
	"gez/internal/custom"
	"gez/internal/git"
	"gez/internal/ui"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

var vscodeCmd = &cobra.Command{
	Use:   "vscode",
	Short: "VSCode .vscode/tasks.json 생성 (커스텀 명령어 → VSCode 태스크)",
	Long: `현재 프로젝트의 gez 커스텀 명령어를 VSCode tasks.json으로 내보냅니다.
생성 후 VSCode에서 Ctrl+Shift+B (빌드) 또는 Ctrl+Shift+P → "Run Task"로 실행할 수 있습니다.

커스텀 명령어가 없을 경우 'gez custom detect'를 먼저 실행하세요.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runVSCode()
	},
}

type vsTask struct {
	Label   string            `json:"label"`
	Type    string            `json:"type"`
	Command string            `json:"command"`
	Args    []string          `json:"args,omitempty"`
	Group   any               `json:"group,omitempty"`
	Options map[string]string `json:"options,omitempty"`
	Detail  string            `json:"detail,omitempty"`
}

type vsTasks struct {
	Version string   `json:"version"`
	Tasks   []vsTask `json:"tasks"`
}

func runVSCode() error {
	dir, err := os.Getwd()
	if err != nil {
		return err
	}
	projName := filepath.Base(dir)
	cfg := custom.Load()
	pc := cfg.ForProject(projName)

	// Also check git root as project name
	if len(pc.Commands) == 0 {
		if root, e := gitRootDir(); e == nil {
			projName = filepath.Base(root)
			pc = cfg.ForProject(projName)
		}
	}

	if len(pc.Commands) == 0 {
		ui.Warn("커스텀 명령어 없음 — gez custom detect 를 먼저 실행하세요")
		return nil
	}

	// Build tasks list
	var tasks []vsTask
	firstBuild := true
	for _, c := range pc.Commands {
		grp := c.Group
		task := vsTask{
			Label:   c.Name,
			Type:    "shell",
			Command: c.Cmd(),
			Detail:  c.Description,
			Options: map[string]string{"cwd": "${workspaceFolder}"},
		}
		// Mark build-related commands as the default build task
		lower := strings.ToLower(c.Name)
		isBuild := strings.Contains(lower, "build") || strings.Contains(lower, "compile")
		if grp == "build" || isBuild {
			if firstBuild {
				task.Group = map[string]any{"kind": "build", "isDefault": true}
				firstBuild = false
			} else {
				task.Group = "build"
			}
		} else if strings.Contains(lower, "test") {
			task.Group = "test"
		}
		tasks = append(tasks, task)
	}

	// Add gez web as a utility task
	tasks = append(tasks, vsTask{
		Label:   "gez: Open Web GUI",
		Type:    "shell",
		Command: "gez web",
		Detail:  "gez 웹 GUI 브라우저에서 열기",
		Options: map[string]string{"cwd": "${workspaceFolder}"},
	})
	tasks = append(tasks, vsTask{
		Label:   "gez: custom detect",
		Type:    "shell",
		Command: "gez custom detect",
		Detail:  "커스텀 명령어 자동 감지",
		Options: map[string]string{"cwd": "${workspaceFolder}"},
	})

	payload := vsTasks{Version: "2.0.0", Tasks: tasks}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}

	vsdir := filepath.Join(dir, ".vscode")
	if err := os.MkdirAll(vsdir, 0o755); err != nil {
		return err
	}
	outPath := filepath.Join(vsdir, "tasks.json")

	// Warn if already exists
	if _, err := os.Stat(outPath); err == nil {
		fmt.Printf("  %s  기존 파일을 덮어씁니다: %s\n", ui.Yellow("⚠"), outPath)
	}

	if err := os.WriteFile(outPath, data, 0o644); err != nil {
		return err
	}
	ui.Success(fmt.Sprintf("생성 완료: %s", outPath))
	fmt.Printf("\n  %s 등록된 태스크 (%d개):\n", ui.Dim("→"), len(tasks))
	for _, t := range tasks {
		grpLabel := ""
		switch v := t.Group.(type) {
		case string:
			grpLabel = " [" + v + "]"
		case map[string]any:
			grpLabel = " [build★]"
		case nil:
		}
		fmt.Printf("    %s%s\n", ui.Cyan(t.Label), ui.Dim(grpLabel))
	}
	fmt.Printf("\n  %s VSCode에서 Ctrl+Shift+P → 'Tasks: Run Task' 로 실행하세요\n\n", ui.Dim("ℹ"))
	return nil
}

func gitRootDir() (string, error) {
	out, err := git.Run("rev-parse", "--show-toplevel")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

func init() {
	rootCmd.AddCommand(vscodeCmd)
}
