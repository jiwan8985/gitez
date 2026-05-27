// Package custom manages per-project custom commands for gez TUI / CLI.
package custom

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// ── Types ─────────────────────────────────────────────────────────────────────

// Command is a named runnable command tied to a project.
type Command struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	CmdWin      string `json:"cmd_win,omitempty"`  // PowerShell snippet
	CmdUnix     string `json:"cmd_unix,omitempty"` // bash snippet
	Group       string `json:"group,omitempty"`    // UI grouping header
}

// Cmd returns the platform-appropriate command string.
func (c Command) Cmd() string {
	if runtime.GOOS == "windows" && c.CmdWin != "" {
		return c.CmdWin
	}
	if c.CmdUnix != "" {
		return c.CmdUnix
	}
	return c.CmdWin
}

// ProjectConfig holds custom commands for one project.
type ProjectConfig struct {
	Name     string    `json:"name"`
	Commands []Command `json:"commands"`
}

// Config is the top-level custom commands config stored in
// ~/.config/gez/custom_commands.json
type Config struct {
	Projects []ProjectConfig `json:"projects"`
}

// GroupedCommands is a named group of commands (for TUI rendering).
type GroupedCommands struct {
	Group    string
	Commands []Command
}

// ── Persistence ───────────────────────────────────────────────────────────────

// ConfigPath returns the platform-appropriate path to the custom commands file.
func ConfigPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "gez", "custom_commands.json")
}

// Load reads the config file; missing file → built-in defaults.
// Built-in defaults are merged for any project not present in the user file.
func Load() Config {
	data, err := os.ReadFile(ConfigPath())
	if err != nil {
		return defaultConfig()
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return defaultConfig()
	}
	// Merge built-ins for unknown projects
	known := map[string]bool{}
	for _, p := range cfg.Projects {
		known[strings.ToLower(p.Name)] = true
	}
	for _, p := range defaultConfig().Projects {
		if !known[strings.ToLower(p.Name)] {
			cfg.Projects = append(cfg.Projects, p)
		}
	}
	return cfg
}

// Save persists the config to disk.
func Save(cfg Config) error {
	path := ConfigPath()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// ── Query helpers ─────────────────────────────────────────────────────────────

// ForProject returns the ProjectConfig for the given name (case-insensitive).
func (c Config) ForProject(name string) ProjectConfig {
	lower := strings.ToLower(name)
	for _, p := range c.Projects {
		if strings.ToLower(p.Name) == lower {
			return p
		}
	}
	return ProjectConfig{Name: name}
}

// GroupCommands returns commands in grouped order, preserving declaration order.
func GroupCommands(cmds []Command) []GroupedCommands {
	order := []string{}
	seen := map[string]bool{}
	groups := map[string][]Command{}
	for _, c := range cmds {
		g := c.Group
		if g == "" {
			g = "기타"
		}
		if !seen[g] {
			order = append(order, g)
			seen[g] = true
		}
		groups[g] = append(groups[g], c)
	}
	result := make([]GroupedCommands, 0, len(order))
	for _, g := range order {
		result = append(result, GroupedCommands{Group: g, Commands: groups[g]})
	}
	return result
}

// FlatList returns (groupHeader, command) pairs in display order.
// Group headers have empty Command.Name.
type FlatItem struct {
	IsHeader bool
	Header   string
	Cmd      Command
}

func FlatList(cmds []Command) []FlatItem {
	var items []FlatItem
	for _, g := range GroupCommands(cmds) {
		items = append(items, FlatItem{IsHeader: true, Header: g.Group})
		for _, c := range g.Commands {
			items = append(items, FlatItem{Cmd: c})
		}
	}
	return items
}

// ── Mutations ─────────────────────────────────────────────────────────────────

// AddCommand adds or replaces a command in the project config.
func (c *Config) AddCommand(projectName string, cmd Command) {
	lower := strings.ToLower(projectName)
	for i, p := range c.Projects {
		if strings.ToLower(p.Name) == lower {
			for j, existing := range p.Commands {
				if existing.Name == cmd.Name {
					c.Projects[i].Commands[j] = cmd
					return
				}
			}
			c.Projects[i].Commands = append(c.Projects[i].Commands, cmd)
			return
		}
	}
	c.Projects = append(c.Projects, ProjectConfig{
		Name:     projectName,
		Commands: []Command{cmd},
	})
}

// RemoveCommand removes a command; returns true if found.
func (c *Config) RemoveCommand(projectName, cmdName string) bool {
	lower := strings.ToLower(projectName)
	for i, p := range c.Projects {
		if strings.ToLower(p.Name) == lower {
			before := len(p.Commands)
			cmds := make([]Command, 0, before)
			for _, cmd := range p.Commands {
				if cmd.Name != cmdName {
					cmds = append(cmds, cmd)
				}
			}
			c.Projects[i].Commands = cmds
			return len(cmds) < before
		}
	}
	return false
}

// ── Built-in defaults ─────────────────────────────────────────────────────────

func defaultConfig() Config {
	return Config{Projects: []ProjectConfig{
		builtinChatService(),
		builtinCloudBilling(),
		builtinAwsBillings(),
		builtinNexusPurge(),
		builtinNexonAwsBilling(),
	}}
}

func cmd(name, desc, group, win, unix string) Command {
	return Command{Name: name, Description: desc, Group: group, CmdWin: win, CmdUnix: unix}
}

// ── ChatService ───────────────────────────────────────────────────────────────

func builtinChatService() ProjectConfig {
	return ProjectConfig{Name: "ChatService", Commands: []Command{
		// 서비스
		cmd("dev",            "개발 모드 시작 (hot reload)",           "서비스", `.\\make.ps1 dev`,            `make dev`),
		cmd("start",          "프로덕션 서비스 시작",                  "서비스", `.\\make.ps1 start`,          `make start`),
		cmd("stop",           "서비스 중지",                           "서비스", `.\\make.ps1 stop`,           `make stop`),
		cmd("restart",        "서비스 재시작",                         "서비스", `.\\make.ps1 restart`,        `make restart`),
		cmd("logs",           "서비스 로그 실시간 보기",               "서비스", `.\\make.ps1 logs`,           `make logs`),
		// 빌드
		cmd("build",          "전체 빌드 (frontend + rust)",          "빌드",   `.\\make.ps1 build`,          `make build`),
		cmd("build-backend",  "Rust 백엔드만 빌드",                   "빌드",   `.\\make.ps1 build-backend`,  `make build-backend`),
		cmd("build-frontend", "프론트엔드만 빌드",                     "빌드",   `.\\make.ps1 build-frontend`, `make build-frontend`),
		// 코드 품질
		cmd("check",          "Rust 코드 검사 (cargo check)",         "품질",   `.\\make.ps1 check`,          `make check`),
		cmd("lint",           "Clippy + TSC 린트",                    "품질",   `.\\make.ps1 lint`,           `make lint`),
		cmd("fmt",            "코드 포맷 (cargo fmt + prettier)",     "품질",   `.\\make.ps1 fmt`,            `make fmt`),
		cmd("test",           "테스트 실행 (cargo test)",             "품질",   `.\\make.ps1 test`,           `make test`),
		// 기타
		cmd("install",        "의존성 설치",                           "기타",   `.\\make.ps1 install`,        `make install`),
		cmd("clean",          "빌드 아티팩트 정리",                   "기타",   `.\\make.ps1 clean`,          `make clean`),
	}}
}

// ── cloud-billing ─────────────────────────────────────────────────────────────

func builtinCloudBilling() ProjectConfig {
	return ProjectConfig{Name: "cloud-billing", Commands: []Command{
		// 서비스
		cmd("start",       "core 서비스 시작",               "서비스", `.\\run.ps1 start`,              `make start`),
		cmd("start-all",   "전체 서비스 시작 (monitoring 포함)", "서비스", `.\\run.ps1 start -Profile all`,  `make start profile=all`),
		cmd("stop",        "서비스 중지",                    "서비스", `.\\run.ps1 stop`,               `make stop`),
		cmd("stop-clean",  "서비스 중지 + 볼륨 삭제 (DB 초기화)", "서비스", `.\\run.ps1 stop -Clean`,         `make stop-clean`),
		cmd("restart",     "서비스 재시작 (core)",           "서비스", `.\\run.ps1 stop; .\\run.ps1 start`, `make stop && make start`),
		cmd("ps",          "실행 중인 컨테이너 목록",        "서비스", `.\\run.ps1 ps`,                 `make ps`),
		cmd("logs",        "core 서비스 로그",               "서비스", `.\\run.ps1 logs`,               `make logs`),
		// 빌드
		cmd("build",       "전체 빌드",                      "빌드",   `.\\run.ps1 build`,              `make build`),
		// 기타
		cmd("seed",        "DB 시드 데이터 투입",            "기타",   `.\\run.ps1 seed`,               `make seed`),
	}}
}

// ── AwsBillings ───────────────────────────────────────────────────────────────

func builtinAwsBillings() ProjectConfig {
	return ProjectConfig{Name: "AwsBillings", Commands: []Command{
		// 서비스
		cmd("dev",             "개발 모드 시작 (hot reload)",       "서비스", `.\\make.ps1 dev`,             `make dev`),
		cmd("start",           "프로덕션 서비스 시작",              "서비스", `.\\make.ps1 start`,           `make start`),
		cmd("stop",            "서비스 중지",                       "서비스", `.\\make.ps1 stop`,            `make stop`),
		cmd("restart",         "서비스 재시작 (dev 모드)",          "서비스", `.\\make.ps1 restart`,         `make restart`),
		cmd("logs",            "백엔드 + 프론트 로그",              "서비스", `.\\make.ps1 logs`,            `make logs`),
		cmd("logs-backend",    "백엔드 로그만",                     "서비스", `.\\make.ps1 logs-backend`,    `make logs-backend`),
		cmd("logs-frontend",   "프론트엔드 로그만",                 "서비스", `.\\make.ps1 logs-frontend`,   `make logs-frontend`),
		// 빌드
		cmd("build",           "전체 빌드 (frontend + rust)",      "빌드",   `.\\make.ps1 build`,           `make build`),
		cmd("build-backend",   "Rust 백엔드만 빌드",               "빌드",   `.\\make.ps1 build-backend`,   `make build-backend`),
		cmd("build-frontend",  "프론트엔드만 빌드",                 "빌드",   `.\\make.ps1 build-frontend`,  `make build-frontend`),
		cmd("install",         "의존성 전체 설치",                  "빌드",   `.\\make.ps1 install`,         `make install`),
		// 품질
		cmd("check",           "Cargo check",                       "품질",   `.\\make.ps1 check`,           `make check`),
		cmd("lint",            "Clippy + TSC 린트",                 "품질",   `.\\make.ps1 lint`,            `make lint`),
		cmd("fmt",             "코드 포맷 (cargo fmt + prettier)", "품질",   `.\\make.ps1 fmt`,             `make fmt`),
		cmd("test",            "전체 테스트",                       "품질",   `.\\make.ps1 test`,            `make test`),
		cmd("test-backend",    "Rust 테스트만",                     "품질",   `.\\make.ps1 test-backend`,    `make test-backend`),
		cmd("ci",              "CI 전체 검사 (lint + test)",        "품질",   `.\\make.ps1 ci`,              `make ci`),
		cmd("clean",           "빌드 아티팩트 정리",               "기타",   `.\\make.ps1 clean`,           `make clean`),
	}}
}

// ── NexusPurge ────────────────────────────────────────────────────────────────

func builtinNexusPurge() ProjectConfig {
	return ProjectConfig{Name: "NexusPurge", Commands: []Command{
		// 개발
		cmd("tauri-dev",        "Tauri 개발 모드 (네이티브 창)",     "개발",   `pnpm tauri dev`,             `pnpm tauri dev`),
		cmd("dev",              "웹 개발 서버 (Vite)",               "개발",   `pnpm dev`,                   `pnpm dev`),
		cmd("preview",          "빌드 결과 미리보기",                "개발",   `pnpm preview`,               `pnpm preview`),
		// 빌드
		cmd("tauri-build",      "Tauri 앱 빌드 (배포용)",           "빌드",   `pnpm tauri build`,           `pnpm tauri build`),
		cmd("build",            "웹 빌드 (Vite + TSC)",             "빌드",   `pnpm build`,                 `pnpm build`),
		cmd("typecheck",        "TypeScript 타입 검사",              "빌드",   `pnpm typecheck`,             `pnpm typecheck`),
		// 테스트
		cmd("test",             "테스트 실행 (vitest run)",          "테스트", `pnpm test`,                  `pnpm test`),
		cmd("test-watch",       "테스트 감시 모드",                  "테스트", `pnpm test:watch`,            `pnpm test:watch`),
		cmd("test-localstack",  "LocalStack 통합 테스트",           "테스트", `bash scripts/localstack-integration.sh`, `bash scripts/localstack-integration.sh`),
	}}
}

// ── NexonAwsBilling ───────────────────────────────────────────────────────────

func builtinNexonAwsBilling() ProjectConfig {
	return ProjectConfig{Name: "NexonAwsBilling", Commands: []Command{
		// 서비스
		cmd("start",   "서비스 시작",             "서비스", `.\\make.ps1 start`,   `make start`),
		cmd("stop",    "서비스 중지",             "서비스", `.\\make.ps1 stop`,    `make stop`),
		// 빌드
		cmd("build",   "전체 빌드 (rust + node)", "빌드",   `.\\make.ps1 build`,   `make build`),
		cmd("check",   "코드 검사 + 빌드 확인",   "빌드",   `.\\make.ps1 check`,   `make check`),
		cmd("install", "의존성 설치",              "빌드",   `.\\make.ps1 install`, `make install`),
		cmd("clean",   "빌드 아티팩트 정리",      "기타",   `.\\make.ps1 clean`,   `make clean`),
	}}
}
