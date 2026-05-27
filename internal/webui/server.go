// Package webui implements the HTTP server for gez's web-based Git GUI.
package webui

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"gez/internal/custom"
	"gez/internal/workspace"
)

// ── ANSI stripping ────────────────────────────────────────────────────────────

var ansiRe = regexp.MustCompile(`\x1b(?:[@-Z\\-_]|\[[0-9;]*[0-9A-ORZcf-ntqry=><~])|\x1b\][^\x07]*\x07`)

func stripAnsi(s string) string {
	s = ansiRe.ReplaceAllString(s, "")
	return strings.ReplaceAll(s, "\r", "")
}

// ── Job / SSE streaming ───────────────────────────────────────────────────────

var jobSeq int64

// Job holds the live output of a running subprocess and notifies SSE subscribers.
type Job struct {
	ID     string
	mu     sync.Mutex
	buf    []string
	done   bool
	exit   int
	notify chan struct{}
}

func newJob() *Job {
	id := atomic.AddInt64(&jobSeq, 1)
	return &Job{
		ID:     fmt.Sprintf("job-%d", id),
		notify: make(chan struct{}, 1),
	}
}

func (j *Job) push(line string) {
	j.mu.Lock()
	j.buf = append(j.buf, line)
	j.mu.Unlock()
	j.signal()
}

func (j *Job) finish(exit int) {
	j.mu.Lock()
	j.done = true
	j.exit = exit
	j.mu.Unlock()
	j.signal()
}

func (j *Job) signal() {
	select {
	case j.notify <- struct{}{}:
	default:
	}
}

// ── Server ────────────────────────────────────────────────────────────────────

// Server is the web GUI HTTP server.
type Server struct {
	Dir  string
	Port int
	jobs sync.Map // map[string]*Job
}

// NewServer creates a server rooted at dir.
func NewServer(dir string, port int) *Server {
	abs, err := filepath.Abs(dir)
	if err != nil {
		abs = dir
	}
	return &Server{Dir: abs, Port: port}
}

// ListenAndServe starts the HTTP server.
func (s *Server) ListenAndServe() error {
	addr := fmt.Sprintf(":%d", s.Port)
	return http.ListenAndServe(addr, s.Handler())
}

// Handler builds and returns the HTTP mux.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /", s.handleIndex)
	mux.HandleFunc("GET /api/status", s.handleStatus)
	mux.HandleFunc("GET /api/branches", s.handleBranches)
	mux.HandleFunc("GET /api/commits", s.handleCommits)
	mux.HandleFunc("GET /api/diff", s.handleDiff)
	mux.HandleFunc("POST /api/stage", s.handleStage)
	mux.HandleFunc("POST /api/unstage", s.handleUnstage)
	mux.HandleFunc("POST /api/discard", s.handleDiscard)
	mux.HandleFunc("POST /api/commit", s.handleCommit)
	mux.HandleFunc("POST /api/fetch", s.handleFetch)
	mux.HandleFunc("POST /api/pull", s.handlePull)
	mux.HandleFunc("POST /api/push", s.handlePush)
	mux.HandleFunc("POST /api/branch/switch", s.handleBranchSwitch)
	mux.HandleFunc("POST /api/branch/create", s.handleBranchCreate)
	mux.HandleFunc("DELETE /api/branch", s.handleBranchDelete)
	mux.HandleFunc("GET /api/stash", s.handleStashList)
	mux.HandleFunc("POST /api/stash/push", s.handleStashPush)
	mux.HandleFunc("POST /api/stash/pop", s.handleStashPop)
	mux.HandleFunc("POST /api/stash/drop", s.handleStashDrop)
	mux.HandleFunc("GET /api/custom", s.handleCustomList)
	mux.HandleFunc("POST /api/run", s.handleRun)
	mux.HandleFunc("GET /api/stream/{id}", s.handleStream)

	return mux
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func jsonOK(w http.ResponseWriter, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(map[string]any{"ok": true, "data": data})
}

func jsonErr(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]any{"ok": false, "error": msg})
}

// git runs a git command in s.Dir and returns trimmed stdout.
func (s *Server) git(args ...string) (string, error) {
	all := append([]string{"-C", s.Dir}, args...)
	c := exec.Command("git", all...)
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

// ── Static ────────────────────────────────────────────────────────────────────

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(indexHTML)
}

// ── Status ────────────────────────────────────────────────────────────────────

type fileStatus struct {
	XY      string `json:"xy"`
	Path    string `json:"path"`
	OldPath string `json:"old_path,omitempty"`
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	branch, _ := s.git("branch", "--show-current")
	if branch == "" {
		branch, _ = s.git("rev-parse", "--short", "HEAD")
		if branch != "" {
			branch = "HEAD:" + branch
		}
	}

	ahead, behind := 0, 0
	if out, err := s.git("rev-list", "--left-right", "--count", "HEAD...@{upstream}"); err == nil {
		parts := strings.Fields(out)
		if len(parts) == 2 {
			ahead, _ = strconv.Atoi(parts[0])
			behind, _ = strconv.Atoi(parts[1])
		}
	}

	var staged, unstaged []fileStatus
	if out, _ := s.git("status", "--porcelain=v1"); out != "" {
		for _, line := range strings.Split(out, "\n") {
			if len(line) < 3 {
				continue
			}
			xy := line[:2]
			path := line[3:]
			oldPath := ""
			if strings.Contains(path, " -> ") {
				parts := strings.SplitN(path, " -> ", 2)
				oldPath = strings.Trim(parts[0], "\"")
				path = strings.Trim(parts[1], "\"")
			}
			path = strings.Trim(path, "\"")
			x, y := xy[0], xy[1]
			if x != ' ' && x != '?' {
				staged = append(staged, fileStatus{XY: xy, Path: path, OldPath: oldPath})
			}
			if y != ' ' {
				unstaged = append(unstaged, fileStatus{XY: xy, Path: path})
			}
		}
	}
	if staged == nil {
		staged = []fileStatus{}
	}
	if unstaged == nil {
		unstaged = []fileStatus{}
	}

	repoName := filepath.Base(s.Dir)

	jsonOK(w, map[string]any{
		"branch":    branch,
		"ahead":     ahead,
		"behind":    behind,
		"staged":    staged,
		"unstaged":  unstaged,
		"repo_name": repoName,
	})
}

// ── Branches ──────────────────────────────────────────────────────────────────

func (s *Server) handleBranches(w http.ResponseWriter, r *http.Request) {
	current, _ := s.git("branch", "--show-current")

	local := []string{}
	if out, _ := s.git("branch", "--format=%(refname:short)"); out != "" {
		for _, l := range strings.Split(out, "\n") {
			if l = strings.TrimSpace(l); l != "" {
				local = append(local, l)
			}
		}
	}

	remote := []string{}
	if out, _ := s.git("branch", "-r", "--format=%(refname:short)"); out != "" {
		for _, l := range strings.Split(out, "\n") {
			l = strings.TrimSpace(l)
			if l != "" && !strings.Contains(l, "HEAD") {
				remote = append(remote, l)
			}
		}
	}

	jsonOK(w, map[string]any{"local": local, "remote": remote, "current": current})
}

// ── Commits ───────────────────────────────────────────────────────────────────

func (s *Server) handleCommits(w http.ResponseWriter, r *http.Request) {
	n := r.URL.Query().Get("n")
	if n == "" {
		n = "80"
	}
	format := "--pretty=format:%H|%h|%s|%an|%cd|%D"
	out, err := s.git("log", "--date=short", "--decorate=short", format, "-n", n)
	if err != nil {
		jsonOK(w, []any{})
		return
	}

	type commit struct {
		Hash    string `json:"hash"`
		Short   string `json:"short"`
		Subject string `json:"subject"`
		Author  string `json:"author"`
		Date    string `json:"date"`
		Refs    string `json:"refs"`
	}
	var commits []commit
	for _, line := range strings.Split(out, "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "|", 6)
		if len(parts) < 6 {
			continue
		}
		commits = append(commits, commit{
			Hash:    parts[0],
			Short:   parts[1],
			Subject: parts[2],
			Author:  parts[3],
			Date:    parts[4],
			Refs:    parts[5],
		})
	}
	if commits == nil {
		commits = []commit{}
	}
	jsonOK(w, commits)
}

// ── Diff ──────────────────────────────────────────────────────────────────────

func (s *Server) handleDiff(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	staged := r.URL.Query().Get("staged") == "true"
	hash := r.URL.Query().Get("hash")

	var out string
	var err error
	switch {
	case hash != "":
		out, err = s.git("show", hash)
	case path == "" && staged:
		out, err = s.git("diff", "--cached")
	case path == "" && !staged:
		out, err = s.git("diff")
	case staged:
		out, err = s.git("diff", "--cached", "--", path)
	default:
		out, err = s.git("diff", "--", path)
	}
	if err != nil {
		out = err.Error()
	}
	jsonOK(w, map[string]string{"content": out})
}

// ── Stage / Unstage / Discard ─────────────────────────────────────────────────

func (s *Server) handleStage(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Path string `json:"path"`
		All  bool   `json:"all"`
	}
	json.NewDecoder(r.Body).Decode(&body)

	var err error
	if body.All {
		_, err = s.git("add", "-A")
	} else if body.Path != "" {
		_, err = s.git("add", "--", body.Path)
	} else {
		jsonErr(w, 400, "path or all required")
		return
	}
	if err != nil {
		jsonErr(w, 500, err.Error())
		return
	}
	jsonOK(w, nil)
}

func (s *Server) handleUnstage(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Path string `json:"path"`
		All  bool   `json:"all"`
	}
	json.NewDecoder(r.Body).Decode(&body)

	var err error
	if body.All {
		_, err = s.git("reset", "HEAD")
	} else if body.Path != "" {
		_, err = s.git("reset", "HEAD", "--", body.Path)
	} else {
		jsonErr(w, 400, "path or all required")
		return
	}
	if err != nil {
		jsonErr(w, 500, err.Error())
		return
	}
	jsonOK(w, nil)
}

func (s *Server) handleDiscard(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Path string `json:"path"`
	}
	json.NewDecoder(r.Body).Decode(&body)
	if body.Path == "" {
		jsonErr(w, 400, "path required")
		return
	}
	_, err := s.git("checkout", "--", body.Path)
	if err != nil {
		jsonErr(w, 500, err.Error())
		return
	}
	jsonOK(w, nil)
}

// ── Commit ────────────────────────────────────────────────────────────────────

func (s *Server) handleCommit(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Message string `json:"message"`
		Amend   bool   `json:"amend"`
	}
	json.NewDecoder(r.Body).Decode(&body)

	args := []string{"commit"}
	if body.Amend {
		args = append(args, "--amend")
		if body.Message != "" {
			args = append(args, "-m", body.Message)
		} else {
			args = append(args, "--no-edit")
		}
	} else {
		if body.Message == "" {
			jsonErr(w, 400, "message required")
			return
		}
		args = append(args, "-m", body.Message)
	}

	out, err := s.git(args...)
	if err != nil {
		jsonErr(w, 500, err.Error())
		return
	}
	jsonOK(w, map[string]string{"output": out})
}

// ── Job runner ────────────────────────────────────────────────────────────────

func (s *Server) startGitJob(args ...string) *Job {
	j := newJob()
	s.jobs.Store(j.ID, j)
	go func() {
		all := append([]string{"-C", s.Dir}, args...)
		cmd := exec.Command("git", all...)
		s.execJob(j, cmd)
	}()
	return j
}

func (s *Server) execJob(j *Job, cmd *exec.Cmd) {
	stdout, _ := cmd.StdoutPipe()
	stderr, _ := cmd.StderrPipe()

	if err := cmd.Start(); err != nil {
		j.push("error: " + err.Error())
		j.finish(1)
		return
	}

	var wg sync.WaitGroup
	wg.Add(2)
	scan := func(rd io.Reader) {
		defer wg.Done()
		sc := bufio.NewScanner(rd)
		sc.Buffer(make([]byte, 512*1024), 512*1024)
		for sc.Scan() {
			j.push(stripAnsi(sc.Text()))
		}
	}
	go scan(stdout)
	go scan(stderr)
	wg.Wait()

	err := cmd.Wait()
	exit := 0
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			exit = ee.ExitCode()
		} else {
			exit = 1
		}
	}
	j.finish(exit)
}

// ── Remote operations ─────────────────────────────────────────────────────────

func (s *Server) handleFetch(w http.ResponseWriter, r *http.Request) {
	j := s.startGitJob("fetch", "--all", "--prune")
	jsonOK(w, map[string]string{"id": j.ID})
}

func (s *Server) handlePull(w http.ResponseWriter, r *http.Request) {
	j := s.startGitJob("pull")
	jsonOK(w, map[string]string{"id": j.ID})
}

func (s *Server) handlePush(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Force       bool `json:"force"`
		SetUpstream bool `json:"set_upstream"`
	}
	json.NewDecoder(r.Body).Decode(&body)

	args := []string{"push"}
	if body.Force {
		args = append(args, "--force-with-lease")
	}
	if body.SetUpstream {
		if branch, _ := s.git("branch", "--show-current"); branch != "" {
			args = append(args, "--set-upstream", "origin", branch)
		}
	}

	j := s.startGitJob(args...)
	jsonOK(w, map[string]string{"id": j.ID})
}

// ── Branch operations ─────────────────────────────────────────────────────────

func (s *Server) handleBranchSwitch(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name string `json:"name"`
	}
	json.NewDecoder(r.Body).Decode(&body)
	if body.Name == "" {
		jsonErr(w, 400, "name required")
		return
	}
	_, err := s.git("checkout", body.Name)
	if err != nil {
		jsonErr(w, 500, err.Error())
		return
	}
	jsonOK(w, nil)
}

func (s *Server) handleBranchCreate(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name   string `json:"name"`
		Switch bool   `json:"switch"`
	}
	json.NewDecoder(r.Body).Decode(&body)
	if body.Name == "" {
		jsonErr(w, 400, "name required")
		return
	}
	var args []string
	if body.Switch {
		args = []string{"checkout", "-b", body.Name}
	} else {
		args = []string{"branch", body.Name}
	}
	_, err := s.git(args...)
	if err != nil {
		jsonErr(w, 500, err.Error())
		return
	}
	jsonOK(w, nil)
}

func (s *Server) handleBranchDelete(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name  string `json:"name"`
		Force bool   `json:"force"`
	}
	json.NewDecoder(r.Body).Decode(&body)
	if body.Name == "" {
		jsonErr(w, 400, "name required")
		return
	}
	flag := "-d"
	if body.Force {
		flag = "-D"
	}
	_, err := s.git("branch", flag, body.Name)
	if err != nil {
		jsonErr(w, 500, err.Error())
		return
	}
	jsonOK(w, nil)
}

// ── Stash ─────────────────────────────────────────────────────────────────────

func (s *Server) handleStashList(w http.ResponseWriter, r *http.Request) {
	stashes := []string{}
	if out, _ := s.git("stash", "list"); out != "" {
		for _, l := range strings.Split(out, "\n") {
			if l = strings.TrimSpace(l); l != "" {
				stashes = append(stashes, l)
			}
		}
	}
	jsonOK(w, stashes)
}

func (s *Server) handleStashPush(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Message string `json:"message"`
	}
	json.NewDecoder(r.Body).Decode(&body)
	args := []string{"stash", "push"}
	if body.Message != "" {
		args = append(args, "-m", body.Message)
	}
	_, err := s.git(args...)
	if err != nil {
		jsonErr(w, 500, err.Error())
		return
	}
	jsonOK(w, nil)
}

func (s *Server) handleStashPop(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Ref string `json:"ref"`
	}
	json.NewDecoder(r.Body).Decode(&body)
	args := []string{"stash", "pop"}
	if body.Ref != "" {
		args = append(args, body.Ref)
	}
	_, err := s.git(args...)
	if err != nil {
		jsonErr(w, 500, err.Error())
		return
	}
	jsonOK(w, nil)
}

func (s *Server) handleStashDrop(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Ref string `json:"ref"`
	}
	json.NewDecoder(r.Body).Decode(&body)
	args := []string{"stash", "drop"}
	if body.Ref != "" {
		args = append(args, body.Ref)
	}
	_, err := s.git(args...)
	if err != nil {
		jsonErr(w, 500, err.Error())
		return
	}
	jsonOK(w, nil)
}

// ── Custom commands ───────────────────────────────────────────────────────────

func (s *Server) handleCustomList(w http.ResponseWriter, r *http.Request) {
	projName := r.URL.Query().Get("project")
	if projName == "" {
		// Try workspace lookup
		if ws, err := workspace.Load(); err == nil {
			for _, p := range ws.Projects {
				if abs, _ := filepath.Abs(p.Path); abs == s.Dir {
					projName = p.Name
					break
				}
			}
		}
		if projName == "" {
			projName = filepath.Base(s.Dir)
		}
	}

	cfg := custom.Load()
	pc := cfg.ForProject(projName)

	cmds := pc.Commands
	if cmds == nil {
		cmds = []custom.Command{}
	}

	jsonOK(w, map[string]any{
		"project":  projName,
		"commands": cmds,
	})
}

// ── Run ───────────────────────────────────────────────────────────────────────

type runRequest struct {
	Type    string   `json:"type"`    // "custom" | "git" | "shell"
	Name    string   `json:"name"`    // custom command name
	Project string   `json:"project"` // custom project
	Args    []string `json:"args"`    // git args
	Cmd     string   `json:"cmd"`     // shell command
}

func (s *Server) handleRun(w http.ResponseWriter, r *http.Request) {
	var req runRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonErr(w, 400, "invalid body")
		return
	}

	j := newJob()
	s.jobs.Store(j.ID, j)

	switch req.Type {
	case "custom":
		projName := req.Project
		if projName == "" {
			projName = filepath.Base(s.Dir)
		}
		cfg := custom.Load()
		pc := cfg.ForProject(projName)
		var found *custom.Command
		for i := range pc.Commands {
			if pc.Commands[i].Name == req.Name {
				found = &pc.Commands[i]
				break
			}
		}
		if found == nil {
			s.jobs.Delete(j.ID)
			jsonErr(w, 404, "command not found: "+req.Name)
			return
		}
		cmdStr := found.Cmd()
		go s.runShellJob(j, cmdStr)

	case "git":
		go func() {
			all := append([]string{"-C", s.Dir}, req.Args...)
			cmd := exec.Command("git", all...)
			s.execJob(j, cmd)
		}()

	case "shell":
		go s.runShellJob(j, req.Cmd)

	default:
		s.jobs.Delete(j.ID)
		jsonErr(w, 400, "unknown type: "+req.Type)
		return
	}

	jsonOK(w, map[string]string{"id": j.ID})
}

func (s *Server) runShellJob(j *Job, cmdStr string) {
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", cmdStr)
	} else {
		cmd = exec.Command("sh", "-c", cmdStr)
	}
	cmd.Dir = s.Dir
	s.execJob(j, cmd)
}

// ── SSE stream ────────────────────────────────────────────────────────────────

func (s *Server) handleStream(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	// Retry briefly: job registered before ID returned, but network can be fast.
	var val any
	var ok bool
	for i := 0; i < 20; i++ {
		val, ok = s.jobs.Load(id)
		if ok {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if !ok {
		http.Error(w, "job not found", http.StatusNotFound)
		return
	}
	j := val.(*Job)

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	pos := 0
	ctx := r.Context()
	for {
		j.mu.Lock()
		newLines := make([]string, len(j.buf)-pos)
		copy(newLines, j.buf[pos:])
		pos = len(j.buf)
		isDone := j.done
		exit := j.exit
		j.mu.Unlock()

		for _, line := range newLines {
			data, _ := json.Marshal(map[string]any{"type": "output", "data": line})
			fmt.Fprintf(w, "data: %s\n\n", data)
		}

		if isDone {
			data, _ := json.Marshal(map[string]any{"type": "done", "exit": exit})
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
			return
		}

		flusher.Flush()

		select {
		case <-ctx.Done():
			return
		case <-j.notify:
		}
	}
}
