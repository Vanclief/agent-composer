package controller

import (
	"strings"
	"time"

	"github.com/vanclief/ez"
)

// DefaultGitHubPoll is how often the GitHub trigger source polls when the
// config file does not say (ADR 0014).
const DefaultGitHubPoll = 60 * time.Second

// minGitHubPoll keeps a typo from spending the token's rate limit.
const minGitHubPoll = 10 * time.Second

// TriggersSettings is the triggers: block of the config file: outside
// sources whose events agc posts as messages (ADR 0014).
type TriggersSettings struct {
	GitHub GitHubTriggers `mapstructure:"github" json:"github"`
}

// GitHubTriggers is triggers.github: the repos to poll (owner/name), the
// logins whose events are dropped before posting, and the poll interval
// as a Go duration. No repos means the source is off.
type GitHubTriggers struct {
	Repos       []string `mapstructure:"repos" json:"repos"`
	IgnoreUsers []string `mapstructure:"ignore_users" json:"ignore_users"`
	Poll        string   `mapstructure:"poll" json:"poll"`
}

// Enabled reports whether any repo is configured.
func (g GitHubTriggers) Enabled() bool {
	return len(g.RepoList()) > 0
}

// RepoList returns the configured repos trimmed, without empty entries.
func (g GitHubTriggers) RepoList() []string {
	repos := []string{}
	for _, repo := range g.Repos {
		trimmed := strings.TrimSpace(repo)
		if trimmed != "" {
			repos = append(repos, trimmed)
		}
	}

	return repos
}

// Validate checks every repo is owner/name and the poll interval parses.
func (g GitHubTriggers) Validate() error {
	for _, repo := range g.RepoList() {
		owner, name, found := strings.Cut(repo, "/")
		if !found || owner == "" || name == "" || strings.Contains(name, "/") {
			return ez.New(ez.EINVALID, "triggers.github.repos: "+repo+" is not owner/name", nil)
		}
	}

	_, err := g.PollInterval()
	return err
}

// PollInterval parses poll, DefaultGitHubPoll when empty, at least 10s.
func (g GitHubTriggers) PollInterval() (time.Duration, error) {
	raw := strings.TrimSpace(g.Poll)
	if raw == "" {
		return DefaultGitHubPoll, nil
	}

	every, err := time.ParseDuration(raw)
	if err != nil {
		return 0, ez.New(ez.EINVALID, "triggers.github.poll: "+raw+" is not a duration such as 60s", err)
	}

	if every < minGitHubPoll {
		return 0, ez.New(ez.EINVALID, "triggers.github.poll: "+raw+" is shorter than 10s", nil)
	}

	return every, nil
}
