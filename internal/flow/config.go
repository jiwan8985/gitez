// Package flow manages Git branching strategy configuration.
// Settings are stored in the repository's git config (local scope)
// under the [gez "flow"] section, making them per-project.
package flow

import (
	"fmt"
	"gez/internal/git"
	"strings"
)

// Strategy represents the chosen branching strategy.
type Strategy string

const (
	StrategyGitFlow    Strategy = "gitflow"
	StrategyGitHubFlow Strategy = "githubflow"
	StrategyTrunk      Strategy = "trunk"
	StrategyNone       Strategy = ""
)

// Config holds the flow configuration for a repository.
type Config struct {
	Strategy Strategy

	// Branch names
	MainBranch    string // production branch (default: main)
	DevelopBranch string // integration branch, git-flow only (default: develop)

	// Prefixes (git-flow / github-flow)
	FeaturePrefix string // default: feature/
	ReleasePrefix string // default: release/
	HotfixPrefix  string // default: hotfix/
	SupportPrefix string // default: support/
	TagPrefix     string // default: v
}

// gitKey returns the git config key for a given field name.
func gitKey(field string) string {
	return fmt.Sprintf("gez.flow.%s", field)
}

// Load reads flow config from the repository's local git config.
// Returns an empty Config (StrategyNone) if not yet initialised.
func Load() (*Config, error) {
	get := func(field string) string {
		out, _ := git.Run("config", "--local", "--get", gitKey(field))
		return strings.TrimSpace(out)
	}

	strategy := Strategy(get("strategy"))
	if strategy == StrategyNone {
		return &Config{Strategy: StrategyNone}, nil
	}

	cfg := &Config{
		Strategy:      strategy,
		MainBranch:    get("main"),
		DevelopBranch: get("develop"),
		FeaturePrefix: get("featurePrefix"),
		ReleasePrefix: get("releasePrefix"),
		HotfixPrefix:  get("hotfixPrefix"),
		SupportPrefix: get("supportPrefix"),
		TagPrefix:     get("tagPrefix"),
	}

	// Fallback defaults
	if cfg.MainBranch == "" {
		cfg.MainBranch = "main"
	}
	if cfg.DevelopBranch == "" {
		cfg.DevelopBranch = "develop"
	}
	if cfg.FeaturePrefix == "" {
		cfg.FeaturePrefix = "feature/"
	}
	if cfg.ReleasePrefix == "" {
		cfg.ReleasePrefix = "release/"
	}
	if cfg.HotfixPrefix == "" {
		cfg.HotfixPrefix = "hotfix/"
	}
	if cfg.SupportPrefix == "" {
		cfg.SupportPrefix = "support/"
	}
	if cfg.TagPrefix == "" {
		cfg.TagPrefix = "v"
	}

	return cfg, nil
}

// Save writes config to the repository's local git config.
func (c *Config) Save() error {
	set := func(field, value string) error {
		_, err := git.Run("config", "--local", gitKey(field), value)
		return err
	}

	pairs := []struct{ k, v string }{
		{"strategy", string(c.Strategy)},
		{"main", c.MainBranch},
		{"develop", c.DevelopBranch},
		{"featurePrefix", c.FeaturePrefix},
		{"releasePrefix", c.ReleasePrefix},
		{"hotfixPrefix", c.HotfixPrefix},
		{"supportPrefix", c.SupportPrefix},
		{"tagPrefix", c.TagPrefix},
	}
	for _, p := range pairs {
		if err := set(p.k, p.v); err != nil {
			return fmt.Errorf("git config 저장 실패 (%s): %w", p.k, err)
		}
	}
	return nil
}

// IsInitialised reports whether the flow strategy has been set up.
func (c *Config) IsInitialised() bool {
	return c.Strategy != StrategyNone
}

// ── Branch name helpers ───────────────────────────────────────────────────────

func (c *Config) FeatureBranch(name string) string {
	return c.FeaturePrefix + name
}

func (c *Config) ReleaseBranch(version string) string {
	return c.ReleasePrefix + version
}

func (c *Config) HotfixBranch(name string) string {
	return c.HotfixPrefix + name
}

func (c *Config) TagName(version string) string {
	if strings.HasPrefix(version, c.TagPrefix) {
		return version
	}
	return c.TagPrefix + version
}

// ── Strategy display names ────────────────────────────────────────────────────

func (s Strategy) Label() string {
	switch s {
	case StrategyGitFlow:
		return "Git Flow"
	case StrategyGitHubFlow:
		return "GitHub Flow"
	case StrategyTrunk:
		return "Trunk-based Development"
	default:
		return "미설정"
	}
}

// CurrentFlowBranch returns the type of the current branch based on config prefixes.
// Returns (branchType, shortName).
func (c *Config) CurrentFlowBranch() (string, string) {
	current := strings.TrimSpace(func() string {
		out, _ := git.Run("branch", "--show-current")
		return out
	}())

	switch {
	case current == c.MainBranch:
		return "main", current
	case current == c.DevelopBranch:
		return "develop", current
	case strings.HasPrefix(current, c.FeaturePrefix):
		return "feature", strings.TrimPrefix(current, c.FeaturePrefix)
	case strings.HasPrefix(current, c.ReleasePrefix):
		return "release", strings.TrimPrefix(current, c.ReleasePrefix)
	case strings.HasPrefix(current, c.HotfixPrefix):
		return "hotfix", strings.TrimPrefix(current, c.HotfixPrefix)
	case strings.HasPrefix(current, c.SupportPrefix):
		return "support", strings.TrimPrefix(current, c.SupportPrefix)
	default:
		return "other", current
	}
}

// ActiveFeatures returns all local feature branch short names.
func (c *Config) ActiveFeatures() []string {
	return branchesWithPrefix(c.FeaturePrefix)
}

// ActiveReleases returns all local release branch short names.
func (c *Config) ActiveReleases() []string {
	return branchesWithPrefix(c.ReleasePrefix)
}

// ActiveHotfixes returns all local hotfix branch short names.
func (c *Config) ActiveHotfixes() []string {
	return branchesWithPrefix(c.HotfixPrefix)
}

func branchesWithPrefix(prefix string) []string {
	out, err := git.Run("branch", "--format=%(refname:short)")
	if err != nil || out == "" {
		return nil
	}
	var result []string
	for _, b := range strings.Split(out, "\n") {
		b = strings.TrimSpace(b)
		if strings.HasPrefix(b, prefix) {
			result = append(result, strings.TrimPrefix(b, prefix))
		}
	}
	return result
}
