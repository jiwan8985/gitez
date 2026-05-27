package workspace

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Project represents a registered project in the workspace.
type Project struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

// Config holds the workspace configuration.
type Config struct {
	Projects []Project `json:"projects"`
}

// configPath returns the path to the config file (~/.config/gez/projects.json).
func configPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("홈 디렉토리를 찾을 수 없습니다: %w", err)
	}
	dir := filepath.Join(home, ".config", "gez")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("설정 디렉토리 생성 실패: %w", err)
	}
	return filepath.Join(dir, "projects.json"), nil
}

// Load reads the workspace config from disk. Returns an empty config on first run.
func Load() (*Config, error) {
	path, err := configPath()
	if err != nil {
		return &Config{}, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Config{}, nil
		}
		return nil, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return &Config{}, nil // corrupt config → start fresh
	}
	return &cfg, nil
}

// Save writes the config to disk.
func (c *Config) Save() error {
	path, err := configPath()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// Add registers a new project. Uses current directory if path is empty.
func (c *Config) Add(path string) error {
	if path == "" {
		var err error
		path, err = os.Getwd()
		if err != nil {
			return err
		}
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	// Check if already registered
	for _, p := range c.Projects {
		if p.Path == abs {
			return fmt.Errorf("'%s' 은(는) 이미 '%s' 이름으로 등록되어 있습니다", HomePath(abs), p.Name)
		}
	}
	name := c.uniqueName(filepath.Base(abs))
	c.Projects = append(c.Projects, Project{Name: name, Path: abs})
	return c.Save()
}

// Rename renames a registered project.
func (c *Config) Rename(oldName, newName string) error {
	for _, p := range c.Projects {
		if p.Name == newName {
			return fmt.Errorf("'%s' 이름이 이미 사용 중입니다", newName)
		}
	}
	for i, p := range c.Projects {
		if p.Name == oldName {
			c.Projects[i].Name = newName
			return c.Save()
		}
	}
	return fmt.Errorf("'%s' 프로젝트를 찾을 수 없습니다", oldName)
}

// uniqueName returns a name that doesn't conflict with existing project names.
func (c *Config) uniqueName(base string) string {
	exists := func(n string) bool {
		for _, p := range c.Projects {
			if p.Name == n {
				return true
			}
		}
		return false
	}
	if !exists(base) {
		return base
	}
	for i := 2; ; i++ {
		candidate := fmt.Sprintf("%s-%d", base, i)
		if !exists(candidate) {
			return candidate
		}
	}
}

// Remove removes a project by name or path suffix.
func (c *Config) Remove(nameOrPath string) error {
	for i, p := range c.Projects {
		slash := filepath.ToSlash(p.Path)
		if p.Name == nameOrPath || p.Path == nameOrPath ||
			strings.HasSuffix(slash, "/"+nameOrPath) {
			c.Projects = append(c.Projects[:i], c.Projects[i+1:]...)
			return c.Save()
		}
	}
	return fmt.Errorf("'%s' 프로젝트를 찾을 수 없습니다\n  목록 확인: gez ws ls", nameOrPath)
}

// Find returns the project matching the given name or exact path (nil if not found).
func (c *Config) Find(nameOrPath string) *Project {
	for i, p := range c.Projects {
		if p.Name == nameOrPath || p.Path == nameOrPath {
			return &c.Projects[i]
		}
	}
	return nil
}

// HomePath replaces the home directory prefix with ~ for display.
func HomePath(path string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	// Normalize separators for comparison
	normPath := filepath.ToSlash(path)
	normHome := filepath.ToSlash(home)
	if strings.HasPrefix(normPath, normHome) {
		return "~" + normPath[len(normHome):]
	}
	return path
}
