// Package webui implements the HTTP server for gez's web-based Git GUI.
package webui

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
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
	"gez/internal/git"
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

// ── gitCtx — per-request git context (supports ?dir= param) ──────────────────

type gitCtx struct {
	dir string
	s   *Server
}

// ctx returns a gitCtx for this request, using ?dir= if present and valid.
func (s *Server) ctx(r *http.Request) gitCtx {
	return gitCtx{dir: s.workDir(r), s: s}
}

// workDir resolves the working directory from ?dir= query param.
// Falls back to s.Dir if the param is absent or not a valid git repo.
func (s *Server) workDir(r *http.Request) string {
	dir := r.URL.Query().Get("dir")
	if dir == "" {
		return s.Dir
	}
	// Expand leading ~
	if strings.HasPrefix(dir, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			dir = filepath.Join(home, dir[2:])
		}
	} else if dir == "~" {
		if home, err := os.UserHomeDir(); err == nil {
			dir = home
		}
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return s.Dir
	}
	// Validate that abs is a git repository
	c := exec.Command("git", "-C", abs, "rev-parse", "--git-dir")
	if err := c.Run(); err != nil {
		return s.Dir
	}
	return abs
}

// run executes a git command in g.dir and returns trimmed stdout.
func (g gitCtx) run(args ...string) (string, error) {
	all := append([]string{"-C", g.dir}, args...)
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

// startJob runs a git command as a streaming job.
func (g gitCtx) startJob(args ...string) *Job {
	j := newJob()
	g.s.jobs.Store(j.ID, j)
	go func() {
		all := append([]string{"-C", g.dir}, args...)
		cmd := exec.Command("git", all...)
		g.s.execJob(j, cmd)
	}()
	return j
}

// startShellJob runs a shell command as a streaming job in g.dir.
func (g gitCtx) startShellJob(cmdStr string) *Job {
	j := newJob()
	g.s.jobs.Store(j.ID, j)
	go func() {
		var cmd *exec.Cmd
		if runtime.GOOS == "windows" {
			cmd = exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", cmdStr)
		} else {
			cmd = exec.Command("sh", "-c", cmdStr)
		}
		cmd.Dir = g.dir
		g.s.execJob(j, cmd)
	}()
	return j
}

func (g gitCtx) repoName() string {
	return filepath.Base(g.dir)
}

// ── Server ────────────────────────────────────────────────────────────────────

// Server is the web GUI HTTP server.
type Server struct {
	Dir       string
	Port      int
	GezBinary string    // path to the gez executable (for running built-in commands)
	jobs      sync.Map  // map[string]*Job
}

// NewServer creates a server rooted at dir.
func NewServer(dir string, port int) *Server {
	abs, err := filepath.Abs(dir)
	if err != nil {
		abs = dir
	}
	s := &Server{Dir: abs, Port: port}
	if exe, err := os.Executable(); err == nil {
		s.GezBinary = exe
	} else {
		s.GezBinary = "gez"
	}
	return s
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
	// Tags
	mux.HandleFunc("GET /api/tags", s.handleTagList)
	mux.HandleFunc("POST /api/tag", s.handleTagCreate)
	mux.HandleFunc("DELETE /api/tag", s.handleTagDelete)
	mux.HandleFunc("POST /api/tag/push", s.handleTagPush)
	// Misc
	mux.HandleFunc("POST /api/cherry-pick", s.handleCherryPick)
	mux.HandleFunc("GET /api/commit-template", s.handleCommitTemplate)
	mux.HandleFunc("POST /api/undo", s.handleUndo)
	mux.HandleFunc("GET /api/workspace", s.handleWorkspace)
	mux.HandleFunc("POST /api/stage/hunk", s.handleStageHunk)
	mux.HandleFunc("GET /api/diff/stat", s.handleDiffStat)
	mux.HandleFunc("POST /api/revert", s.handleRevert)
	mux.HandleFunc("POST /api/reset-to", s.handleResetTo)
	mux.HandleFunc("POST /api/branch/rename", s.handleBranchRename)
	mux.HandleFunc("POST /api/merge", s.handleMerge)
	mux.HandleFunc("GET /api/blame", s.handleBlame)
	mux.HandleFunc("GET /api/commit/{hash}", s.handleCommitDetail)
	mux.HandleFunc("GET /api/stash/show", s.handleStashShow)
	mux.HandleFunc("GET /api/file/log", s.handleFileLog)
	// New endpoints
	mux.HandleFunc("GET /api/repos", s.handleRepos)
	mux.HandleFunc("GET /api/browse", s.handleBrowse)
	mux.HandleFunc("GET /api/gez-commands", s.handleGezCommands)
	mux.HandleFunc("POST /api/rebase", s.handleRebase)
	mux.HandleFunc("POST /api/squash", s.handleSquash)
	mux.HandleFunc("POST /api/clean", s.handleClean)
	mux.HandleFunc("POST /api/branch/from-commit", s.handleBranchFromCommit)

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

// git runs a git command in s.Dir (server default dir). Used for server-level operations.
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
	g := s.ctx(r)

	branch, _ := g.run("branch", "--show-current")
	if branch == "" {
		branch, _ = g.run("rev-parse", "--short", "HEAD")
		if branch != "" {
			branch = "HEAD:" + branch
		}
	}

	ahead, behind := 0, 0
	if out, err := g.run("rev-list", "--left-right", "--count", "HEAD...@{upstream}"); err == nil {
		parts := strings.Fields(out)
		if len(parts) == 2 {
			ahead, _ = strconv.Atoi(parts[0])
			behind, _ = strconv.Atoi(parts[1])
		}
	}

	var staged, unstaged []fileStatus
	if out, _ := g.run("status", "--porcelain=v1"); out != "" {
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

	jsonOK(w, map[string]any{
		"branch":    branch,
		"ahead":     ahead,
		"behind":    behind,
		"staged":    staged,
		"unstaged":  unstaged,
		"repo_name": g.repoName(),
		"dir":       g.dir,
	})
}

// ── Branches ──────────────────────────────────────────────────────────────────

func (s *Server) handleBranches(w http.ResponseWriter, r *http.Request) {
	g := s.ctx(r)
	current, _ := g.run("branch", "--show-current")

	local := []string{}
	if out, _ := g.run("branch", "--format=%(refname:short)"); out != "" {
		for _, l := range strings.Split(out, "\n") {
			if l = strings.TrimSpace(l); l != "" {
				local = append(local, l)
			}
		}
	}

	remote := []string{}
	if out, _ := g.run("branch", "-r", "--format=%(refname:short)"); out != "" {
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
	g := s.ctx(r)
	n := r.URL.Query().Get("n")
	if n == "" {
		n = "200"
	}
	format := "--pretty=format:%H|%h|%s|%an|%ae|%cd|%D|%P"
	var out string
	var err error
	if n == "all" {
		out, err = g.run("log", "--date=short", "--decorate=short", format)
	} else {
		out, err = g.run("log", "--date=short", "--decorate=short", format, "-n", n)
	}
	if err != nil {
		jsonOK(w, []any{})
		return
	}

	type commit struct {
		Hash    string   `json:"hash"`
		Short   string   `json:"short"`
		Subject string   `json:"subject"`
		Author  string   `json:"author"`
		Email   string   `json:"email"`
		Date    string   `json:"date"`
		Refs    string   `json:"refs"`
		Parents []string `json:"parents"`
	}
	var commits []commit
	for _, line := range strings.Split(out, "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "|", 8)
		if len(parts) < 8 {
			continue
		}
		parents := []string{}
		if parts[7] != "" {
			parents = strings.Fields(parts[7])
		}
		commits = append(commits, commit{
			Hash:    parts[0],
			Short:   parts[1],
			Subject: parts[2],
			Author:  parts[3],
			Email:   parts[4],
			Date:    parts[5],
			Refs:    parts[6],
			Parents: parents,
		})
	}
	if commits == nil {
		commits = []commit{}
	}
	jsonOK(w, commits)
}

// ── Diff ──────────────────────────────────────────────────────────────────────

func (s *Server) handleDiff(w http.ResponseWriter, r *http.Request) {
	g := s.ctx(r)
	path := r.URL.Query().Get("path")
	staged := r.URL.Query().Get("staged") == "true"
	hash := r.URL.Query().Get("hash")

	var out string
	var err error
	switch {
	case hash != "" && path != "":
		out, err = g.run("show", hash, "--", path)
	case hash != "":
		out, err = g.run("show", hash)
	case path == "" && staged:
		out, err = g.run("diff", "--cached")
	case path == "" && !staged:
		out, err = g.run("diff")
	case staged:
		out, err = g.run("diff", "--cached", "--", path)
	default:
		out, err = g.run("diff", "--", path)
	}
	if err != nil {
		out = err.Error()
	}
	jsonOK(w, map[string]string{"content": out})
}

// ── Stage / Unstage / Discard ─────────────────────────────────────────────────

func (s *Server) handleStage(w http.ResponseWriter, r *http.Request) {
	g := s.ctx(r)
	var body struct {
		Path string `json:"path"`
		All  bool   `json:"all"`
	}
	json.NewDecoder(r.Body).Decode(&body)

	var err error
	if body.All {
		_, err = g.run("add", "-A")
	} else if body.Path != "" {
		_, err = g.run("add", "--", body.Path)
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
	g := s.ctx(r)
	var body struct {
		Path string `json:"path"`
		All  bool   `json:"all"`
	}
	json.NewDecoder(r.Body).Decode(&body)

	var err error
	if body.All {
		_, err = g.run("reset", "HEAD")
	} else if body.Path != "" {
		_, err = g.run("reset", "HEAD", "--", body.Path)
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
	g := s.ctx(r)
	var body struct {
		Path string `json:"path"`
	}
	json.NewDecoder(r.Body).Decode(&body)
	if body.Path == "" {
		jsonErr(w, 400, "path required")
		return
	}
	_, err := g.run("checkout", "--", body.Path)
	if err != nil {
		jsonErr(w, 500, err.Error())
		return
	}
	jsonOK(w, nil)
}

// ── Commit ────────────────────────────────────────────────────────────────────

func (s *Server) handleCommit(w http.ResponseWriter, r *http.Request) {
	g := s.ctx(r)
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

	out, err := g.run(args...)
	if err != nil {
		jsonErr(w, 500, err.Error())
		return
	}
	jsonOK(w, map[string]string{"output": out})
}

// ── Job runner ────────────────────────────────────────────────────────────────

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
	g := s.ctx(r)
	j := g.startJob("fetch", "--all", "--prune")
	jsonOK(w, map[string]string{"id": j.ID})
}

func (s *Server) handlePull(w http.ResponseWriter, r *http.Request) {
	g := s.ctx(r)
	var body struct {
		Rebase bool `json:"rebase"`
	}
	json.NewDecoder(r.Body).Decode(&body)
	args := []string{"pull"}
	if body.Rebase {
		args = append(args, "--rebase")
	}
	j := g.startJob(args...)
	jsonOK(w, map[string]string{"id": j.ID})
}

func (s *Server) handlePush(w http.ResponseWriter, r *http.Request) {
	g := s.ctx(r)
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
		if branch, _ := g.run("branch", "--show-current"); branch != "" {
			args = append(args, "--set-upstream", "origin", branch)
		}
	}

	j := g.startJob(args...)
	jsonOK(w, map[string]string{"id": j.ID})
}

// ── Branch operations ─────────────────────────────────────────────────────────

func (s *Server) handleBranchSwitch(w http.ResponseWriter, r *http.Request) {
	g := s.ctx(r)
	var body struct {
		Name string `json:"name"`
	}
	json.NewDecoder(r.Body).Decode(&body)
	if body.Name == "" {
		jsonErr(w, 400, "name required")
		return
	}
	_, err := g.run("checkout", body.Name)
	if err != nil {
		jsonErr(w, 500, err.Error())
		return
	}
	jsonOK(w, nil)
}

func (s *Server) handleBranchCreate(w http.ResponseWriter, r *http.Request) {
	g := s.ctx(r)
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
	_, err := g.run(args...)
	if err != nil {
		jsonErr(w, 500, err.Error())
		return
	}
	jsonOK(w, nil)
}

func (s *Server) handleBranchDelete(w http.ResponseWriter, r *http.Request) {
	g := s.ctx(r)
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
	_, err := g.run("branch", flag, body.Name)
	if err != nil {
		jsonErr(w, 500, err.Error())
		return
	}
	jsonOK(w, nil)
}

func (s *Server) handleBranchRename(w http.ResponseWriter, r *http.Request) {
	g := s.ctx(r)
	var body struct {
		Old string `json:"old"`
		New string `json:"new"`
	}
	json.NewDecoder(r.Body).Decode(&body)
	if body.Old == "" || body.New == "" {
		jsonErr(w, 400, "old and new required")
		return
	}
	_, err := g.run("branch", "-m", body.Old, body.New)
	if err != nil {
		jsonErr(w, 500, err.Error())
		return
	}
	jsonOK(w, nil)
}

func (s *Server) handleBranchFromCommit(w http.ResponseWriter, r *http.Request) {
	g := s.ctx(r)
	var body struct {
		Name string `json:"name"`
		Hash string `json:"hash"`
	}
	json.NewDecoder(r.Body).Decode(&body)
	if body.Name == "" || body.Hash == "" {
		jsonErr(w, 400, "name and hash required")
		return
	}
	_, err := g.run("branch", body.Name, body.Hash)
	if err != nil {
		jsonErr(w, 500, err.Error())
		return
	}
	jsonOK(w, nil)
}

// ── Stash ─────────────────────────────────────────────────────────────────────

func (s *Server) handleStashList(w http.ResponseWriter, r *http.Request) {
	g := s.ctx(r)
	stashes := []string{}
	if out, _ := g.run("stash", "list"); out != "" {
		for _, l := range strings.Split(out, "\n") {
			if l = strings.TrimSpace(l); l != "" {
				stashes = append(stashes, l)
			}
		}
	}
	jsonOK(w, stashes)
}

func (s *Server) handleStashPush(w http.ResponseWriter, r *http.Request) {
	g := s.ctx(r)
	var body struct {
		Message string `json:"message"`
	}
	json.NewDecoder(r.Body).Decode(&body)
	args := []string{"stash", "push"}
	if body.Message != "" {
		args = append(args, "-m", body.Message)
	}
	_, err := g.run(args...)
	if err != nil {
		jsonErr(w, 500, err.Error())
		return
	}
	jsonOK(w, nil)
}

func (s *Server) handleStashPop(w http.ResponseWriter, r *http.Request) {
	g := s.ctx(r)
	var body struct {
		Ref string `json:"ref"`
	}
	json.NewDecoder(r.Body).Decode(&body)
	args := []string{"stash", "pop"}
	if body.Ref != "" {
		args = append(args, body.Ref)
	}
	_, err := g.run(args...)
	if err != nil {
		jsonErr(w, 500, err.Error())
		return
	}
	jsonOK(w, nil)
}

func (s *Server) handleStashDrop(w http.ResponseWriter, r *http.Request) {
	g := s.ctx(r)
	var body struct {
		Ref string `json:"ref"`
	}
	json.NewDecoder(r.Body).Decode(&body)
	args := []string{"stash", "drop"}
	if body.Ref != "" {
		args = append(args, body.Ref)
	}
	_, err := g.run(args...)
	if err != nil {
		jsonErr(w, 500, err.Error())
		return
	}
	jsonOK(w, nil)
}

func (s *Server) handleStashShow(w http.ResponseWriter, r *http.Request) {
	g := s.ctx(r)
	ref := r.URL.Query().Get("ref")
	if ref == "" {
		ref = "stash@{0}"
	}
	out, err := g.run("stash", "show", "-p", ref)
	if err != nil {
		jsonErr(w, 500, err.Error())
		return
	}
	jsonOK(w, map[string]string{"content": out})
}

// ── Custom commands ───────────────────────────────────────────────────────────

func (s *Server) handleCustomList(w http.ResponseWriter, r *http.Request) {
	g := s.ctx(r)
	projName := r.URL.Query().Get("project")
	if projName == "" {
		if ws, err := workspace.Load(); err == nil {
			for _, p := range ws.Projects {
				if abs, _ := filepath.Abs(p.Path); abs == g.dir {
					projName = p.Name
					break
				}
			}
		}
		if projName == "" {
			projName = g.repoName()
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
	Type    string   `json:"type"`    // "custom" | "git" | "shell" | "builtin"
	Name    string   `json:"name"`    // custom command name
	Project string   `json:"project"` // custom project
	Args    []string `json:"args"`    // git args or builtin args (e.g. ["flow","status"])
	Cmd     string   `json:"cmd"`     // shell command
}

func (s *Server) handleRun(w http.ResponseWriter, r *http.Request) {
	g := s.ctx(r)
	var req runRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonErr(w, 400, "invalid body")
		return
	}

	switch req.Type {
	case "custom":
		projName := req.Project
		if projName == "" {
			projName = g.repoName()
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
			jsonErr(w, 404, "command not found: "+req.Name)
			return
		}
		j := g.startShellJob(found.Cmd())
		jsonOK(w, map[string]string{"id": j.ID})

	case "git":
		j := g.startJob(req.Args...)
		jsonOK(w, map[string]string{"id": j.ID})

	case "shell":
		j := g.startShellJob(req.Cmd)
		jsonOK(w, map[string]string{"id": j.ID})

	case "builtin":
		// Run the gez binary with the given subcommand args (non-interactive output only)
		if len(req.Args) == 0 {
			jsonErr(w, 400, "args required for builtin")
			return
		}
		j := newJob()
		s.jobs.Store(j.ID, j)
		go func() {
			cmd := exec.Command(s.GezBinary, req.Args...)
			cmd.Dir = g.dir
			s.execJob(j, cmd)
		}()
		jsonOK(w, map[string]string{"id": j.ID})

	default:
		jsonErr(w, 400, "unknown type: "+req.Type)
	}
}

// ── SSE stream ────────────────────────────────────────────────────────────────

func (s *Server) handleStream(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

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

// ── Tags ──────────────────────────────────────────────────────────────────────

type tagEntry struct {
	Name    string `json:"name"`
	Hash    string `json:"hash"`
	Message string `json:"message"`
	Date    string `json:"date"`
}

func (s *Server) handleTagList(w http.ResponseWriter, r *http.Request) {
	g := s.ctx(r)
	out, _ := g.run("tag", "--sort=-version:refname", "--format=%(refname:short)|%(objectname:short)|%(subject)|%(creatordate:short)")
	var tags []tagEntry
	if out != "" {
		for _, line := range strings.Split(out, "\n") {
			parts := strings.SplitN(line, "|", 4)
			if len(parts) < 4 {
				continue
			}
			tags = append(tags, tagEntry{
				Name:    parts[0],
				Hash:    parts[1],
				Message: parts[2],
				Date:    parts[3],
			})
		}
	}
	if tags == nil {
		tags = []tagEntry{}
	}
	jsonOK(w, tags)
}

func (s *Server) handleTagCreate(w http.ResponseWriter, r *http.Request) {
	g := s.ctx(r)
	var body struct {
		Name    string `json:"name"`
		Message string `json:"message"`
		Hash    string `json:"hash"`
	}
	json.NewDecoder(r.Body).Decode(&body)
	if body.Name == "" {
		jsonErr(w, 400, "name required")
		return
	}
	ref := body.Hash
	if ref == "" {
		ref = "HEAD"
	}
	var args []string
	if body.Message != "" {
		args = []string{"tag", "-a", body.Name, ref, "-m", body.Message}
	} else {
		args = []string{"tag", body.Name, ref}
	}
	_, err := g.run(args...)
	if err != nil {
		jsonErr(w, 500, err.Error())
		return
	}
	jsonOK(w, nil)
}

func (s *Server) handleTagDelete(w http.ResponseWriter, r *http.Request) {
	g := s.ctx(r)
	var body struct {
		Name string `json:"name"`
	}
	json.NewDecoder(r.Body).Decode(&body)
	if body.Name == "" {
		jsonErr(w, 400, "name required")
		return
	}
	_, err := g.run("tag", "-d", body.Name)
	if err != nil {
		jsonErr(w, 500, err.Error())
		return
	}
	jsonOK(w, nil)
}

func (s *Server) handleTagPush(w http.ResponseWriter, r *http.Request) {
	g := s.ctx(r)
	var body struct {
		Name string `json:"name"`
	}
	json.NewDecoder(r.Body).Decode(&body)
	var args []string
	if body.Name != "" {
		args = []string{"push", "origin", body.Name}
	} else {
		args = []string{"push", "origin", "--tags"}
	}
	j := g.startJob(args...)
	jsonOK(w, map[string]string{"id": j.ID})
}

// ── Cherry-pick ───────────────────────────────────────────────────────────────

func (s *Server) handleCherryPick(w http.ResponseWriter, r *http.Request) {
	g := s.ctx(r)
	var body struct {
		Hash string `json:"hash"`
	}
	json.NewDecoder(r.Body).Decode(&body)
	if body.Hash == "" {
		jsonErr(w, 400, "hash required")
		return
	}
	j := g.startJob("cherry-pick", body.Hash)
	jsonOK(w, map[string]string{"id": j.ID})
}

// ── Commit template ───────────────────────────────────────────────────────────

func (s *Server) handleCommitTemplate(w http.ResponseWriter, r *http.Request) {
	g := s.ctx(r)
	templatePath, _ := g.run("config", "commit.template")
	if templatePath != "" {
		if strings.HasPrefix(templatePath, "~") {
			home, _ := os.UserHomeDir()
			templatePath = filepath.Join(home, templatePath[1:])
		}
		data, err := os.ReadFile(templatePath)
		if err == nil {
			jsonOK(w, map[string]string{"template": string(data)})
			return
		}
	}
	home, _ := os.UserHomeDir()
	data, err := os.ReadFile(filepath.Join(home, ".gitmessage"))
	if err == nil {
		jsonOK(w, map[string]string{"template": string(data)})
		return
	}
	jsonOK(w, map[string]string{"template": ""})
}

// ── Undo ──────────────────────────────────────────────────────────────────────

func (s *Server) handleUndo(w http.ResponseWriter, r *http.Request) {
	g := s.ctx(r)
	out, err := g.run("reflog", "show", "--format=%H %gs", "-n", "2")
	if err != nil || out == "" {
		jsonErr(w, 500, "reflog unavailable")
		return
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) < 2 {
		jsonErr(w, 400, "nothing to undo")
		return
	}
	prevHash := strings.Fields(lines[1])[0]
	j := g.startJob("reset", "--soft", prevHash)
	jsonOK(w, map[string]string{"id": j.ID})
}

// ── Stage hunk ────────────────────────────────────────────────────────────────

func (s *Server) handleStageHunk(w http.ResponseWriter, r *http.Request) {
	g := s.ctx(r)
	var body struct {
		Patch   string `json:"patch"`
		Unstage bool   `json:"unstage"`
	}
	json.NewDecoder(r.Body).Decode(&body)
	if body.Patch == "" {
		jsonErr(w, 400, "patch required")
		return
	}
	args := []string{"-C", g.dir, "apply", "--cached"}
	if body.Unstage {
		args = append(args, "--reverse")
	}
	args = append(args, "-")
	cmd := exec.Command("git", args...)
	cmd.Stdin = strings.NewReader(body.Patch)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		jsonErr(w, 500, msg)
		return
	}
	jsonOK(w, nil)
}

// ── Diff stat ─────────────────────────────────────────────────────────────────

func (s *Server) handleDiffStat(w http.ResponseWriter, r *http.Request) {
	g := s.ctx(r)
	hash := r.URL.Query().Get("hash")
	if hash == "" {
		jsonErr(w, 400, "hash required")
		return
	}
	out, err := g.run("diff-tree", "--no-commit-id", "-r", "--name-status", hash)
	if err != nil {
		jsonErr(w, 500, err.Error())
		return
	}
	type fileStat struct {
		Status string `json:"status"`
		Path   string `json:"path"`
	}
	var files []fileStat
	for _, line := range strings.Split(out, "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 2)
		if len(parts) < 2 {
			continue
		}
		files = append(files, fileStat{Status: strings.TrimSpace(parts[0]), Path: strings.TrimSpace(parts[1])})
	}
	if files == nil {
		files = []fileStat{}
	}
	jsonOK(w, files)
}

// ── Revert ────────────────────────────────────────────────────────────────────

func (s *Server) handleRevert(w http.ResponseWriter, r *http.Request) {
	g := s.ctx(r)
	var body struct {
		Hash string `json:"hash"`
	}
	json.NewDecoder(r.Body).Decode(&body)
	if body.Hash == "" {
		jsonErr(w, 400, "hash required")
		return
	}
	j := g.startJob("revert", "--no-edit", body.Hash)
	jsonOK(w, map[string]string{"id": j.ID})
}

// ── Reset to commit ───────────────────────────────────────────────────────────

func (s *Server) handleResetTo(w http.ResponseWriter, r *http.Request) {
	g := s.ctx(r)
	var body struct {
		Hash string `json:"hash"`
		Mode string `json:"mode"`
	}
	json.NewDecoder(r.Body).Decode(&body)
	if body.Hash == "" {
		jsonErr(w, 400, "hash required")
		return
	}
	mode := body.Mode
	if mode != "soft" && mode != "mixed" && mode != "hard" {
		mode = "mixed"
	}
	_, err := g.run("reset", "--"+mode, body.Hash)
	if err != nil {
		jsonErr(w, 500, err.Error())
		return
	}
	jsonOK(w, nil)
}

// ── Merge ─────────────────────────────────────────────────────────────────────

func (s *Server) handleMerge(w http.ResponseWriter, r *http.Request) {
	g := s.ctx(r)
	var body struct {
		Branch  string `json:"branch"`
		Squash  bool   `json:"squash"`
		NoFF    bool   `json:"no_ff"`
		Message string `json:"message"`
	}
	json.NewDecoder(r.Body).Decode(&body)
	if body.Branch == "" {
		jsonErr(w, 400, "branch required")
		return
	}
	args := []string{"merge"}
	if body.Squash {
		args = append(args, "--squash")
	} else if body.NoFF {
		args = append(args, "--no-ff")
	}
	if body.Message != "" {
		args = append(args, "-m", body.Message)
	}
	args = append(args, body.Branch)
	j := g.startJob(args...)
	jsonOK(w, map[string]string{"id": j.ID})
}

// ── Commit detail ─────────────────────────────────────────────────────────────

func (s *Server) handleCommitDetail(w http.ResponseWriter, r *http.Request) {
	g := s.ctx(r)
	hash := r.PathValue("hash")
	if hash == "" {
		jsonErr(w, 400, "hash required")
		return
	}
	msg, _    := g.run("log", "-1", "--format=%B", hash)
	author, _ := g.run("log", "-1", "--format=%an", hash)
	email, _  := g.run("log", "-1", "--format=%ae", hash)
	date, _   := g.run("log", "-1", "--format=%ai", hash)
	stats, _  := g.run("diff-tree", "--no-commit-id", "-r", "--stat", hash)
	jsonOK(w, map[string]any{
		"message": strings.TrimSpace(msg),
		"author":  strings.TrimSpace(author),
		"email":   strings.TrimSpace(email),
		"date":    strings.TrimSpace(date),
		"stats":   strings.TrimSpace(stats),
	})
}

// ── Blame ─────────────────────────────────────────────────────────────────────

func (s *Server) handleBlame(w http.ResponseWriter, r *http.Request) {
	g := s.ctx(r)
	path := r.URL.Query().Get("path")
	if path == "" {
		jsonErr(w, 400, "path required")
		return
	}
	out, err := g.run("blame", "-p", "--", path)
	if err != nil {
		jsonErr(w, 500, err.Error())
		return
	}
	jsonOK(w, map[string]string{"content": out})
}

// ── File log ──────────────────────────────────────────────────────────────────

func (s *Server) handleFileLog(w http.ResponseWriter, r *http.Request) {
	g := s.ctx(r)
	path := r.URL.Query().Get("path")
	if path == "" {
		jsonErr(w, 400, "path required")
		return
	}
	n := r.URL.Query().Get("n")
	if n == "" {
		n = "100"
	}
	format := "--pretty=format:%H|%h|%s|%an|%cd"
	out, err := g.run("log", "--date=short", "--follow", format, "-n", n, "--", path)
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
	}
	var commits []commit
	for _, line := range strings.Split(out, "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "|", 5)
		if len(parts) < 5 {
			continue
		}
		commits = append(commits, commit{
			Hash: parts[0], Short: parts[1], Subject: parts[2],
			Author: parts[3], Date: parts[4],
		})
	}
	if commits == nil {
		commits = []commit{}
	}
	jsonOK(w, commits)
}

// ── Workspace overview ────────────────────────────────────────────────────────

type wsProject struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	Branch  string `json:"branch"`
	Ahead   int    `json:"ahead"`
	Behind  int    `json:"behind"`
	Changed int    `json:"changed"`
	Valid   bool   `json:"valid"`
}

func (s *Server) handleWorkspace(w http.ResponseWriter, r *http.Request) {
	ws, err := workspace.Load()
	if err != nil || ws == nil {
		jsonOK(w, []wsProject{})
		return
	}
	projects := make([]wsProject, 0, len(ws.Projects))
	for _, p := range ws.Projects {
		if !git.IsRepoInDir(p.Path) {
			projects = append(projects, wsProject{Name: p.Name, Path: p.Path})
			continue
		}
		branch := git.CurrentBranchInDir(p.Path)
		aheadStr, behindStr := git.AheadBehindInDir(p.Path)
		ahead, _ := strconv.Atoi(aheadStr)
		behind, _ := strconv.Atoi(behindStr)
		lines := git.StatusShortInDir(p.Path)
		changed := 0
		for _, l := range lines {
			if len(l) >= 3 {
				changed++
			}
		}
		projects = append(projects, wsProject{
			Name:    p.Name,
			Path:    p.Path,
			Branch:  branch,
			Ahead:   ahead,
			Behind:  behind,
			Changed: changed,
			Valid:   true,
		})
	}
	jsonOK(w, projects)
}

// ── Repos (folder picker) ─────────────────────────────────────────────────────

func (s *Server) handleRepos(w http.ResponseWriter, r *http.Request) {
	type repoEntry struct {
		Name string `json:"name"`
		Path string `json:"path"`
	}
	var repos []repoEntry
	// Server default dir first
	repos = append(repos, repoEntry{Name: filepath.Base(s.Dir), Path: s.Dir})
	// Workspace projects
	if ws, err := workspace.Load(); err == nil {
		for _, p := range ws.Projects {
			abs, err := filepath.Abs(p.Path)
			if err != nil {
				abs = p.Path
			}
			if abs == s.Dir {
				continue
			}
			if git.IsRepoInDir(abs) {
				repos = append(repos, repoEntry{Name: p.Name, Path: abs})
			}
		}
	}
	jsonOK(w, repos)
}

// ── Rebase ────────────────────────────────────────────────────────────────────

func (s *Server) handleRebase(w http.ResponseWriter, r *http.Request) {
	g := s.ctx(r)
	var body struct {
		Onto string `json:"onto"`
	}
	json.NewDecoder(r.Body).Decode(&body)
	if body.Onto == "" {
		jsonErr(w, 400, "onto required")
		return
	}
	j := g.startJob("rebase", body.Onto)
	jsonOK(w, map[string]string{"id": j.ID})
}

// ── Squash ────────────────────────────────────────────────────────────────────

func (s *Server) handleSquash(w http.ResponseWriter, r *http.Request) {
	g := s.ctx(r)
	var body struct {
		N       int    `json:"n"`
		Message string `json:"message"`
	}
	json.NewDecoder(r.Body).Decode(&body)
	if body.N < 2 || body.Message == "" {
		jsonErr(w, 400, "n >= 2 and message required")
		return
	}

	j := newJob()
	g.s.jobs.Store(j.ID, j)
	go func() {
		ref := fmt.Sprintf("HEAD~%d", body.N)
		_, err := g.run("reset", "--soft", ref)
		if err != nil {
			j.push("error: " + err.Error())
			j.finish(1)
			return
		}
		j.push(fmt.Sprintf("reset --soft HEAD~%d 완료", body.N))
		out, err := g.run("commit", "-m", body.Message)
		if err != nil {
			j.push("error: " + err.Error())
			j.finish(1)
			return
		}
		j.push(out)
		j.finish(0)
	}()
	jsonOK(w, map[string]string{"id": j.ID})
}

// ── Clean ─────────────────────────────────────────────────────────────────────

func (s *Server) handleClean(w http.ResponseWriter, r *http.Request) {
	g := s.ctx(r)
	var body struct {
		Force bool `json:"force"`
		Dirs  bool `json:"dirs"`
	}
	json.NewDecoder(r.Body).Decode(&body)

	args := []string{"clean"}
	if body.Force {
		args = append(args, "-f")
	} else {
		args = append(args, "-n") // dry run
	}
	if body.Dirs {
		args = append(args, "-d")
	}

	if !body.Force {
		// Dry run — return output synchronously
		out, err := g.run(args...)
		if err != nil {
			jsonErr(w, 500, err.Error())
			return
		}
		jsonOK(w, map[string]string{"output": out})
		return
	}

	j := g.startJob(args...)
	jsonOK(w, map[string]string{"id": j.ID})
}

// ── Browse (directory picker) ──────────────────────────────────────────────────

func (s *Server) handleBrowse(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	if path == "" {
		if home, err := os.UserHomeDir(); err == nil {
			path = home
		} else {
			path = "/"
		}
	}
	if strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			path = filepath.Join(home, path[2:])
		}
	} else if path == "~" {
		if home, err := os.UserHomeDir(); err == nil {
			path = home
		}
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		jsonErr(w, 400, err.Error())
		return
	}
	entries, err := os.ReadDir(abs)
	if err != nil {
		jsonErr(w, 500, err.Error())
		return
	}
	type browseEntry struct {
		Name  string `json:"name"`
		Path  string `json:"path"`
		IsGit bool   `json:"is_git"`
	}
	var dirs []browseEntry
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		fullPath := filepath.Join(abs, name)
		cmd := exec.Command("git", "-C", fullPath, "rev-parse", "--git-dir")
		cmd.Stdout = io.Discard
		cmd.Stderr = io.Discard
		isGit := cmd.Run() == nil
		dirs = append(dirs, browseEntry{Name: name, Path: fullPath, IsGit: isGit})
	}
	parent := filepath.Dir(abs)
	if parent == abs {
		parent = ""
	}
	cmd := exec.Command("git", "-C", abs, "rev-parse", "--git-dir")
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	isGit := cmd.Run() == nil
	jsonOK(w, map[string]any{
		"path":   abs,
		"parent": parent,
		"dirs":   dirs,
		"is_git": isGit,
	})
}

// ── Gez built-in commands list ────────────────────────────────────────────────

type gezCmdEntry struct {
	Name  string   `json:"name"`
	Desc  string   `json:"desc"`
	Group string   `json:"group"`
	Args  []string `json:"args"`
}

func (s *Server) handleGezCommands(w http.ResponseWriter, r *http.Request) {
	cmds := []gezCmdEntry{
		// 기본 워크플로우
		{Name: "commit",  Desc: "커밋 마법사 (Conventional Commits)", Group: "워크플로우", Args: []string{"commit"}},
		{Name: "push",    Desc: "원격 푸시", Group: "워크플로우", Args: []string{"push"}},
		{Name: "push -f", Desc: "force-with-lease 강제 푸시", Group: "워크플로우", Args: []string{"push", "-f"}},
		{Name: "pull",    Desc: "원격 풀", Group: "워크플로우", Args: []string{"pull"}},
		{Name: "sync",    Desc: "fetch + pull", Group: "워크플로우", Args: []string{"sync"}},
		{Name: "fetch",   Desc: "fetch --all --prune", Group: "워크플로우", Args: []string{"fetch"}},
		{Name: "status",  Desc: "현재 상태 상세", Group: "워크플로우", Args: []string{"status"}},
		{Name: "diff",    Desc: "변경사항 diff", Group: "워크플로우", Args: []string{"diff"}},
		{Name: "log",     Desc: "커밋 로그", Group: "워크플로우", Args: []string{"log"}},
		// 브랜치 & 히스토리
		{Name: "branch",      Desc: "브랜치 관리 메뉴", Group: "브랜치", Args: []string{"branch"}},
		{Name: "merge",       Desc: "브랜치 병합", Group: "브랜치", Args: []string{"merge"}},
		{Name: "rebase",      Desc: "리베이스", Group: "브랜치", Args: []string{"rebase"}},
		{Name: "cherry-pick", Desc: "다른 브랜치 커밋 가져오기", Group: "브랜치", Args: []string{"cherry-pick"}},
		{Name: "revert",      Desc: "커밋 되돌리기 (히스토리 유지)", Group: "브랜치", Args: []string{"revert"}},
		{Name: "reset",       Desc: "soft·mixed·hard reset", Group: "브랜치", Args: []string{"reset"}},
		// 커밋 관리
		{Name: "squash",    Desc: "최근 N개 커밋 합치기", Group: "커밋", Args: []string{"squash"}},
		{Name: "amend",     Desc: "마지막 커밋 수정", Group: "커밋", Args: []string{"amend"}},
		{Name: "fixup",     Desc: "fixup 커밋 + autosquash", Group: "커밋", Args: []string{"fixup"}},
		{Name: "undo",      Desc: "마지막 작업 취소 (reflog 기반)", Group: "커밋", Args: []string{"undo"}},
		{Name: "restore",   Desc: "파일 복원", Group: "커밋", Args: []string{"restore"}},
		{Name: "changelog", Desc: "CHANGELOG.md 생성", Group: "커밋", Args: []string{"changelog"}},
		// 복구 & 정리
		{Name: "stash",  Desc: "스태시 push·pop·apply·drop", Group: "복구", Args: []string{"stash"}},
		{Name: "reflog", Desc: "reflog 조회 + 복구", Group: "복구", Args: []string{"reflog"}},
		{Name: "blame",  Desc: "줄별 작성자·커밋", Group: "복구", Args: []string{"blame"}},
		{Name: "clean",  Desc: "untracked 파일 정리", Group: "복구", Args: []string{"clean"}},
		// 검색 & 분석
		{Name: "search", Desc: "메시지·pickaxe·regex·grep·파일명 검색", Group: "검색", Args: []string{"search"}},
		{Name: "show",   Desc: "커밋 상세 보기", Group: "검색", Args: []string{"show"}},
		{Name: "stats",  Desc: "저장소 통계 (기여자·파일·월별)", Group: "검색", Args: []string{"stats"}},
		{Name: "file",   Desc: "파일별 히스토리·blame·복원", Group: "검색", Args: []string{"file"}},
		{Name: "bisect", Desc: "이진 탐색으로 버그 커밋 찾기", Group: "검색", Args: []string{"bisect"}},
		// 저장소 & 원격
		{Name: "tag",       Desc: "태그 생성·삭제·push", Group: "저장소", Args: []string{"tag"}},
		{Name: "remote",    Desc: "원격 저장소 관리", Group: "저장소", Args: []string{"remote"}},
		{Name: "init",      Desc: "새 git 저장소 초기화", Group: "저장소", Args: []string{"init"}},
		{Name: "clone",     Desc: "저장소 클론", Group: "저장소", Args: []string{"clone"}},
		{Name: "worktree",  Desc: "워크트리 관리", Group: "저장소", Args: []string{"worktree"}},
		{Name: "submodule", Desc: "서브모듈 관리", Group: "저장소", Args: []string{"submodule"}},
		{Name: "pr",        Desc: "PR/MR URL 브라우저 열기", Group: "저장소", Args: []string{"pr"}},
		{Name: "hook",      Desc: "Git hooks 관리", Group: "저장소", Args: []string{"hook"}},
		{Name: "config",    Desc: "Git + gez 설정", Group: "저장소", Args: []string{"config"}},
		{Name: "archive",   Desc: "zip·tar.gz 내보내기", Group: "저장소", Args: []string{"archive"}},
		{Name: "patch",     Desc: "패치 생성·적용", Group: "저장소", Args: []string{"patch"}},
		{Name: "sparse",    Desc: "Sparse checkout (모노레포)", Group: "저장소", Args: []string{"sparse"}},
		// 환경 설정
		{Name: "ignore", Desc: ".gitignore 관리 (12종 템플릿)", Group: "설정", Args: []string{"ignore"}},
		{Name: "alias",  Desc: "git alias 관리", Group: "설정", Args: []string{"alias"}},
		{Name: "doctor", Desc: "Git 환경 진단", Group: "설정", Args: []string{"doctor"}},
		// Flow
		{Name: "flow",                Desc: "Flow 전략 현황 + 힌트", Group: "Flow", Args: []string{"flow"}},
		{Name: "flow init",           Desc: "Flow 전략 초기화", Group: "Flow", Args: []string{"flow", "init"}},
		{Name: "flow feature start",  Desc: "feature 브랜치 시작", Group: "Flow", Args: []string{"flow", "feature", "start"}},
		{Name: "flow feature finish", Desc: "feature 완료", Group: "Flow", Args: []string{"flow", "feature", "finish"}},
		{Name: "flow release start",  Desc: "release 브랜치 시작", Group: "Flow", Args: []string{"flow", "release", "start"}},
		{Name: "flow release finish", Desc: "release 완료", Group: "Flow", Args: []string{"flow", "release", "finish"}},
		{Name: "flow hotfix start",   Desc: "hotfix 시작", Group: "Flow", Args: []string{"flow", "hotfix", "start"}},
		{Name: "flow hotfix finish",  Desc: "hotfix 완료", Group: "Flow", Args: []string{"flow", "hotfix", "finish"}},
		// 워크스페이스
		{Name: "ws",      Desc: "워크스페이스 전체 상태", Group: "워크스페이스", Args: []string{"ws"}},
		{Name: "ws add",  Desc: "프로젝트 등록", Group: "워크스페이스", Args: []string{"ws", "add"}},
		{Name: "ws pull", Desc: "전체 프로젝트 pull", Group: "워크스페이스", Args: []string{"ws", "pull"}},
		{Name: "ws sync", Desc: "전체 프로젝트 fetch + pull", Group: "워크스페이스", Args: []string{"ws", "sync"}},
		// 커스텀
		{Name: "custom",        Desc: "커스텀 명령어 관리", Group: "커스텀", Args: []string{"custom"}},
		{Name: "custom detect", Desc: "프로젝트 파일 자동 분석 → 명령어 등록", Group: "커스텀", Args: []string{"custom", "detect"}},
		{Name: "custom add",    Desc: "커스텀 명령어 추가", Group: "커스텀", Args: []string{"custom", "add"}},
		{Name: "custom ls",     Desc: "커스텀 명령어 목록", Group: "커스텀", Args: []string{"custom", "ls"}},
		{Name: "custom rm",     Desc: "커스텀 명령어 삭제", Group: "커스텀", Args: []string{"custom", "rm"}},
	}
	jsonOK(w, cmds)
}
