// detect.go — 프로젝트 파일을 분석해 커스텀 명령어를 자동 감지합니다.
// 지원 파일: Makefile, make.ps1, run.ps1, package.json, Taskfile.yml,
//            Cargo.toml(Rust 표준), go.mod(Go 표준), docker-compose.yml
package custom

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// ── DetectResult ─────────────────────────────────────────────────────────────

// DetectResult holds auto-detected commands for one project directory.
type DetectResult struct {
	Commands []Command
	Sources  []string // files that were analyzed
}

// ── DetectCommands ───────────────────────────────────────────────────────────

// DetectCommands scans dir for buildscripts and returns detected commands.
// Detection order: PS1 scripts → Makefile → package.json → Taskfile →
// docker-compose → Cargo.toml → go.mod
func DetectCommands(dir string) DetectResult {
	var r DetectResult

	// 1. PowerShell scripts (Windows-native; provide CmdWin)
	for _, script := range []string{"make.ps1", "run.ps1", "Makefile.ps1", "scripts.ps1"} {
		cmds, src := detectPS1(dir, script)
		if len(cmds) > 0 {
			r.Commands = mergeDetected(r.Commands, cmds)
			r.Sources = append(r.Sources, src)
		}
	}

	// 2. Makefile (Unix-native; fills CmdUnix where missing)
	if cmds, src := detectMakefile(dir); len(cmds) > 0 {
		r.Commands = fillUnix(r.Commands, cmds)
		r.Sources = append(r.Sources, src)
	}

	// 3. package.json scripts (cross-platform)
	if cmds, src := detectPackageJSON(dir); len(cmds) > 0 {
		r.Commands = mergeDetected(r.Commands, cmds)
		r.Sources = append(r.Sources, src)
	}

	// 4. Taskfile.yml
	if cmds, src := detectTaskfile(dir); len(cmds) > 0 {
		r.Commands = mergeDetected(r.Commands, cmds)
		r.Sources = append(r.Sources, src)
	}

	// 5. docker-compose.yml (generate up/down/logs/build)
	if cmds, src := detectDockerCompose(dir); len(cmds) > 0 {
		r.Commands = mergeDetected(r.Commands, cmds)
		r.Sources = append(r.Sources, src)
	}

	// 6. Fallback: language standard commands (only if nothing detected yet)
	if len(r.Commands) == 0 {
		if cmds, src := detectLanguageFallback(dir); len(cmds) > 0 {
			r.Commands = cmds
			r.Sources = append(r.Sources, src)
		}
	}

	return r
}

// ── Makefile ─────────────────────────────────────────────────────────────────

var makeTargetRe = regexp.MustCompile(`^([a-zA-Z][a-zA-Z0-9_-]*)\s*:`)

func detectMakefile(dir string) ([]Command, string) {
	path := filepath.Join(dir, "Makefile")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, ""
	}

	// Pre-process: join continuation lines (\<newline>)
	joined := strings.ReplaceAll(string(data), "\\\n", " ")

	phonySet := map[string]bool{}
	allTargets := map[string]bool{}
	scanner := bufio.NewScanner(strings.NewReader(joined))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, ".PHONY:") || strings.HasPrefix(line, ".phony:") {
			for _, t := range strings.Fields(line[7:]) {
				if t != "" && t != "\\" {
					phonySet[t] = true
				}
			}
			continue
		}
		if m := makeTargetRe.FindStringSubmatch(line); m != nil {
			allTargets[m[1]] = true
		}
	}

	// Decide which targets to use
	targets := phonySet
	if len(targets) == 0 {
		targets = allTargets
	}

	var cmds []Command
	for name := range targets {
		if !isValidTarget(name) {
			continue
		}
		cmds = append(cmds, Command{
			Name:        name,
			Description: guessDescription(name),
			Group:       guessGroup(name),
			CmdUnix:     "make " + name,
			CmdWin:      "make " + name,
		})
	}
	if len(cmds) == 0 {
		return nil, ""
	}
	sortCommands(cmds)
	return cmds, "Makefile"
}

// ── PowerShell scripts ────────────────────────────────────────────────────────

var (
	switchCaseRe = regexp.MustCompile(`^\s+"([a-zA-Z][a-zA-Z0-9_-]*)"\s*\{`)
	validateSetRe = regexp.MustCompile(`[Vv]alidate[Ss]et\(([^)]+)\)`)
	ps1FuncRe    = regexp.MustCompile(`(?i)^\s*function\s+([a-zA-Z][a-zA-Z0-9_-]*)`)
)

func detectPS1(dir, filename string) ([]Command, string) {
	path := filepath.Join(dir, filename)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, ""
	}

	scriptRef := `.\` + filename
	seen := map[string]bool{}
	var cmds []Command

	add := func(name string) {
		if seen[name] || !isValidTarget(name) {
			return
		}
		seen[name] = true
		cmds = append(cmds, Command{
			Name:        name,
			Description: guessDescription(name),
			Group:       guessGroup(name),
			CmdWin:      scriptRef + " " + name,
			CmdUnix:     "make " + name, // best-effort Unix fallback
		})
	}

	// ValidateSet attributes expose allowed values
	for _, m := range validateSetRe.FindAllStringSubmatch(string(data), -1) {
		for _, raw := range strings.Split(m[1], ",") {
			add(strings.Trim(strings.TrimSpace(raw), `"'`))
		}
	}

	// Switch/case blocks
	inSwitch := false
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		line := scanner.Text()
		lower := strings.ToLower(strings.TrimSpace(line))
		if strings.Contains(lower, "switch") && (strings.Contains(lower, "$command") ||
			strings.Contains(lower, "$target") || strings.Contains(lower, "$task") ||
			strings.Contains(lower, "$args[0]") || strings.Contains(lower, "$action")) {
			inSwitch = true
		}
		if inSwitch {
			if m := switchCaseRe.FindStringSubmatch(line); m != nil {
				add(m[1])
			}
		}
		// Top-level function definitions (function-based scripts)
		if !inSwitch {
			if m := ps1FuncRe.FindStringSubmatch(line); m != nil {
				name := strings.ToLower(m[1])
				if name != "main" {
					add(name)
				}
			}
		}
	}

	if len(cmds) == 0 {
		return nil, ""
	}
	sortCommands(cmds)
	return cmds, filename
}

// ── package.json ──────────────────────────────────────────────────────────────

// priorityScripts are shown first in the Commands tab.
var priorityScripts = []string{
	"dev", "start", "stop", "restart",
	"build", "build:prod", "build:dev",
	"test", "test:watch", "typecheck",
	"lint", "lint:fix", "format", "fmt", "check",
	"preview", "serve",
	"clean", "install",
}

func detectPackageJSON(dir string) ([]Command, string) {
	path := filepath.Join(dir, "package.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, ""
	}
	var pkg struct {
		Scripts map[string]string `json:"scripts"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil || len(pkg.Scripts) == 0 {
		return nil, ""
	}

	// Detect package manager from lock files
	pm := "npm run"
	for _, lock := range []struct {
		file string
		pm   string
	}{
		{"pnpm-lock.yaml", "pnpm"},
		{"yarn.lock", "yarn"},
		{"bun.lockb", "bun run"},
	} {
		if _, err := os.Stat(filepath.Join(dir, lock.file)); err == nil {
			pm = lock.pm
			break
		}
	}

	seen := map[string]bool{}
	var cmds []Command

	addScript := func(name string) {
		if seen[name] {
			return
		}
		scriptVal, ok := pkg.Scripts[name]
		if !ok {
			return
		}
		seen[name] = true

		runCmd := pm + " " + name
		desc := guessDescription(name)
		if desc == name && scriptVal != "" {
			if len(scriptVal) > 50 {
				scriptVal = scriptVal[:47] + "..."
			}
			desc = scriptVal
		}
		cmds = append(cmds, Command{
			Name:        name,
			Description: desc,
			Group:       guessGroup(name),
			CmdWin:      runCmd,
			CmdUnix:     runCmd,
		})
	}

	// Priority scripts first
	for _, name := range priorityScripts {
		addScript(name)
	}
	// Remaining scripts (sorted for determinism)
	remaining := make([]string, 0, len(pkg.Scripts))
	for name := range pkg.Scripts {
		remaining = append(remaining, name)
	}
	sortStrings(remaining)
	for _, name := range remaining {
		addScript(name)
	}

	if len(cmds) == 0 {
		return nil, ""
	}
	return cmds, "package.json (" + pm + ")"
}

// ── Taskfile.yml ─────────────────────────────────────────────────────────────

func detectTaskfile(dir string) ([]Command, string) {
	for _, name := range []string{"Taskfile.yml", "taskfile.yml", "Taskfile.yaml", "taskfile.yaml"} {
		cmds, src := parseTaskfile(filepath.Join(dir, name), name)
		if len(cmds) > 0 {
			return cmds, src
		}
	}
	return nil, ""
}

var taskNameRe = regexp.MustCompile(`^  ([a-zA-Z][a-zA-Z0-9_:-]*)\s*:`)
var taskDescRe = regexp.MustCompile(`^\s+desc:\s*["']?(.+?)["']?\s*$`)

func parseTaskfile(path, src string) ([]Command, string) {
	f, err := os.Open(path)
	if err != nil {
		return nil, ""
	}
	defer f.Close()

	var cmds []Command
	var last *Command
	inTasks := false

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "tasks:" {
			inTasks = true
			continue
		}
		if !inTasks {
			continue
		}
		if len(line) > 0 && line[0] != ' ' {
			inTasks = false
			continue
		}
		if m := taskNameRe.FindStringSubmatch(line); m != nil {
			name := m[1]
			if isValidTarget(name) {
				cmds = append(cmds, Command{
					Name:        name,
					Description: guessDescription(name),
					Group:       guessGroup(name),
					CmdWin:      "task " + name,
					CmdUnix:     "task " + name,
				})
				last = &cmds[len(cmds)-1]
			}
		} else if last != nil {
			if m := taskDescRe.FindStringSubmatch(line); m != nil {
				last.Description = m[1]
			}
		}
	}
	if len(cmds) == 0 {
		return nil, ""
	}
	return cmds, src
}

// ── docker-compose ────────────────────────────────────────────────────────────

func detectDockerCompose(dir string) ([]Command, string) {
	for _, name := range []string{"docker-compose.yml", "docker-compose.yaml", "compose.yml", "compose.yaml"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
			return buildDockerComposeCommands(name), name
		}
	}
	return nil, ""
}

func buildDockerComposeCommands(filename string) []Command {
	base := "docker compose"
	if strings.HasPrefix(filename, "docker-compose") {
		base = "docker-compose"
	}
	cmds := []Command{
		{Name: "up", Description: "서비스 시작", Group: "서비스",
			CmdWin: base + " up -d", CmdUnix: base + " up -d"},
		{Name: "down", Description: "서비스 중지", Group: "서비스",
			CmdWin: base + " down", CmdUnix: base + " down"},
		{Name: "restart", Description: "서비스 재시작", Group: "서비스",
			CmdWin: base + " restart", CmdUnix: base + " restart"},
		{Name: "logs", Description: "로그 보기", Group: "서비스",
			CmdWin: base + " logs -f", CmdUnix: base + " logs -f"},
		{Name: "ps", Description: "컨테이너 목록", Group: "서비스",
			CmdWin: base + " ps", CmdUnix: base + " ps"},
		{Name: "build", Description: "이미지 빌드", Group: "빌드",
			CmdWin: base + " build", CmdUnix: base + " build"},
		{Name: "pull", Description: "이미지 풀", Group: "빌드",
			CmdWin: base + " pull", CmdUnix: base + " pull"},
	}
	return cmds
}

// ── Language fallbacks ────────────────────────────────────────────────────────

func detectLanguageFallback(dir string) ([]Command, string) {
	// Rust (Cargo.toml)
	if _, err := os.Stat(filepath.Join(dir, "Cargo.toml")); err == nil {
		return cargoCommands(), "Cargo.toml (Rust)"
	}
	// Go (go.mod)
	if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
		return goCommands(), "go.mod (Go)"
	}
	// Python (pyproject.toml / requirements.txt)
	if _, err := os.Stat(filepath.Join(dir, "pyproject.toml")); err == nil {
		return pythonCommands(), "pyproject.toml (Python)"
	}
	return nil, ""
}

func cargoCommands() []Command {
	return []Command{
		{Name: "run", Description: "실행 (debug)", Group: "서비스",
			CmdWin: "cargo run", CmdUnix: "cargo run"},
		{Name: "build", Description: "릴리즈 빌드", Group: "빌드",
			CmdWin: "cargo build --release", CmdUnix: "cargo build --release"},
		{Name: "build-dev", Description: "디버그 빌드", Group: "빌드",
			CmdWin: "cargo build", CmdUnix: "cargo build"},
		{Name: "test", Description: "테스트 실행", Group: "품질",
			CmdWin: "cargo test", CmdUnix: "cargo test"},
		{Name: "check", Description: "코드 검사 (빠름)", Group: "품질",
			CmdWin: "cargo check", CmdUnix: "cargo check"},
		{Name: "clippy", Description: "Clippy 린트", Group: "품질",
			CmdWin: "cargo clippy", CmdUnix: "cargo clippy"},
		{Name: "fmt", Description: "코드 포맷", Group: "품질",
			CmdWin: "cargo fmt", CmdUnix: "cargo fmt"},
		{Name: "clean", Description: "빌드 아티팩트 정리", Group: "기타",
			CmdWin: "cargo clean", CmdUnix: "cargo clean"},
	}
}

func goCommands() []Command {
	return []Command{
		{Name: "run", Description: "실행", Group: "서비스",
			CmdWin: "go run .", CmdUnix: "go run ."},
		{Name: "build", Description: "빌드", Group: "빌드",
			CmdWin: "go build ./...", CmdUnix: "go build ./..."},
		{Name: "test", Description: "테스트 실행", Group: "품질",
			CmdWin: "go test ./...", CmdUnix: "go test ./..."},
		{Name: "vet", Description: "코드 검사", Group: "품질",
			CmdWin: "go vet ./...", CmdUnix: "go vet ./..."},
		{Name: "fmt", Description: "코드 포맷", Group: "품질",
			CmdWin: "gofmt -w .", CmdUnix: "gofmt -w ."},
		{Name: "tidy", Description: "모듈 정리", Group: "기타",
			CmdWin: "go mod tidy", CmdUnix: "go mod tidy"},
	}
}

func pythonCommands() []Command {
	return []Command{
		{Name: "run", Description: "실행", Group: "서비스",
			CmdWin: "python main.py", CmdUnix: "python3 main.py"},
		{Name: "test", Description: "테스트 실행", Group: "품질",
			CmdWin: "pytest", CmdUnix: "pytest"},
		{Name: "install", Description: "의존성 설치", Group: "빌드",
			CmdWin: "pip install -e .", CmdUnix: "pip install -e ."},
		{Name: "lint", Description: "린트 (ruff/flake8)", Group: "품질",
			CmdWin: "ruff check .", CmdUnix: "ruff check ."},
		{Name: "fmt", Description: "포맷 (black/ruff)", Group: "품질",
			CmdWin: "ruff format .", CmdUnix: "ruff format ."},
	}
}

// ── Merge helpers ─────────────────────────────────────────────────────────────

// mergeDetected adds new commands; existing ones have missing fields filled in.
func mergeDetected(existing, newCmds []Command) []Command {
	idx := nameIndex(existing)
	for _, c := range newCmds {
		if i, ok := idx[c.Name]; ok {
			if existing[i].CmdWin == "" && c.CmdWin != "" {
				existing[i].CmdWin = c.CmdWin
			}
			if existing[i].CmdUnix == "" && c.CmdUnix != "" {
				existing[i].CmdUnix = c.CmdUnix
			}
			if existing[i].Description == "" && c.Description != "" {
				existing[i].Description = c.Description
			}
		} else {
			existing = append(existing, c)
			idx[c.Name] = len(existing) - 1
		}
	}
	return existing
}

// fillUnix only fills in missing CmdUnix from the new list.
func fillUnix(existing, newCmds []Command) []Command {
	idx := nameIndex(existing)
	for _, c := range newCmds {
		if i, ok := idx[c.Name]; ok {
			if existing[i].CmdUnix == "" && c.CmdUnix != "" {
				existing[i].CmdUnix = c.CmdUnix
			}
		} else {
			existing = append(existing, c)
			idx[c.Name] = len(existing) - 1
		}
	}
	return existing
}

func nameIndex(cmds []Command) map[string]int {
	m := make(map[string]int, len(cmds))
	for i, c := range cmds {
		m[c.Name] = i
	}
	return m
}

// ── Classification helpers ────────────────────────────────────────────────────

var groupMap = map[string]string{
	"dev": "서비스", "start": "서비스", "stop": "서비스", "restart": "서비스",
	"up": "서비스", "down": "서비스", "run": "서비스", "serve": "서비스",
	"logs": "서비스", "log": "서비스", "ps": "서비스", "preview": "서비스",
	"watch": "서비스",

	"build": "빌드", "compile": "빌드", "install": "빌드", "bundle": "빌드",

	"test": "품질", "lint": "품질", "check": "품질", "fmt": "품질",
	"format": "품질", "typecheck": "품질", "ci": "품질", "clippy": "품질",
	"vet": "품질", "verify": "품질",

	"deploy": "배포", "release": "배포", "publish": "배포",

	"clean": "기타", "purge": "기타", "seed": "기타", "tidy": "기타",
	"pull": "기타",
}

func guessGroup(name string) string {
	lower := strings.ToLower(name)
	// Exact match
	if g, ok := groupMap[lower]; ok {
		return g
	}
	// Prefix or suffix match (e.g. "build-backend", "test-watch")
	for k, g := range groupMap {
		if strings.HasPrefix(lower, k+"-") || strings.HasPrefix(lower, k+":") ||
			strings.HasSuffix(lower, "-"+k) {
			return g
		}
	}
	return "기타"
}

var descMap = map[string]string{
	"dev":       "개발 모드 시작 (hot reload)",
	"start":     "서비스 시작",
	"stop":      "서비스 중지",
	"restart":   "서비스 재시작",
	"up":        "서비스 시작",
	"down":      "서비스 중지",
	"logs":      "로그 보기",
	"log":       "로그 보기",
	"ps":        "컨테이너 목록",
	"run":       "실행",
	"serve":     "서버 시작",
	"preview":   "빌드 미리보기",
	"watch":     "변경 감시 모드",
	"build":     "빌드",
	"compile":   "컴파일",
	"install":   "의존성 설치",
	"bundle":    "번들링",
	"test":      "테스트 실행",
	"lint":      "코드 린트",
	"check":     "코드 검사",
	"fmt":       "코드 포맷",
	"format":    "코드 포맷",
	"typecheck": "TypeScript 타입 검사",
	"ci":        "CI 전체 검사 (lint + test)",
	"clippy":    "Clippy 린트",
	"vet":       "코드 검사 (go vet)",
	"verify":    "검증",
	"deploy":    "배포",
	"release":   "릴리즈 빌드",
	"publish":   "패키지 배포",
	"clean":     "빌드 아티팩트 정리",
	"purge":     "완전 초기화",
	"seed":      "DB 시드 데이터 투입",
	"tidy":      "모듈 정리",
	"pull":      "이미지/의존성 풀",
}

func guessDescription(name string) string {
	lower := strings.ToLower(name)
	if d, ok := descMap[lower]; ok {
		return d
	}
	// Compound names: "build-backend" → "빌드 (backend)"
	parts := strings.SplitN(name, "-", 2)
	if len(parts) == 2 {
		if d, ok := descMap[strings.ToLower(parts[0])]; ok {
			return fmt.Sprintf("%s (%s)", d, parts[1])
		}
	}
	parts = strings.SplitN(name, ":", 2)
	if len(parts) == 2 {
		if d, ok := descMap[strings.ToLower(parts[0])]; ok {
			return fmt.Sprintf("%s (%s)", d, parts[1])
		}
	}
	return name
}

// isValidTarget returns false for targets that are clearly not user-facing commands.
func isValidTarget(name string) bool {
	if len(name) == 0 || len(name) > 40 {
		return false
	}
	// Must start with a letter (no special chars at start)
	first := rune(name[0])
	if !((first >= 'a' && first <= 'z') || (first >= 'A' && first <= 'Z')) {
		return false
	}
	// Skip hidden / internal
	if strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_") {
		return false
	}
	// Must contain only alphanumeric, hyphen, underscore, colon
	for _, r := range name {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') || r == '-' || r == '_' || r == ':') {
			return false
		}
	}
	// Skip all-uppercase (Makefile variables like CC, LDFLAGS, GOPATH)
	upper := true
	for _, r := range name {
		if r >= 'a' && r <= 'z' {
			upper = false
			break
		}
	}
	if upper && len(name) > 2 {
		return false
	}
	// Skip common non-user-facing targets
	skip := map[string]bool{
		"all": true, "default": true, "FORCE": true,
		"main": true, "phony": true, "PHONY": true,
	}
	return !skip[name]
}

// ── Sort helpers ──────────────────────────────────────────────────────────────

// sortCommands sorts by priority group, then alphabetically within group.
func sortCommands(cmds []Command) {
	groupOrder := map[string]int{
		"서비스": 0, "빌드": 1, "품질": 2, "배포": 3, "기타": 4,
	}
	// Simple insertion sort (lists are small)
	for i := 1; i < len(cmds); i++ {
		key := cmds[i]
		j := i - 1
		gi := groupOrder[guessGroup(key.Name)]
		for j >= 0 {
			gj := groupOrder[guessGroup(cmds[j].Name)]
			if gj < gi || (gj == gi && cmds[j].Name <= key.Name) {
				break
			}
			cmds[j+1] = cmds[j]
			j--
		}
		cmds[j+1] = key
	}
}

func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		key := s[i]
		j := i - 1
		for j >= 0 && s[j] > key {
			s[j+1] = s[j]
			j--
		}
		s[j+1] = key
	}
}
