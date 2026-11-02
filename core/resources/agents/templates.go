package agents

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/vanclief/ez"
	"gopkg.in/yaml.v3"

	"github.com/vanclief/agent-composer/models/agent"
	"github.com/vanclief/agent-composer/task"
)

// DefaultIdle is how long an agent waits for a message before it sleeps
// when its template does not say.
const DefaultIdle = 15 * time.Minute

// roleRegex is a role: a template's file name without .yaml, in any case.
var roleRegex = regexp.MustCompile(`^[A-Za-z0-9]+([_-][A-Za-z0-9]+)*$`)

// Template is agents/<role>.yaml (ADR 0016 item 2).
type Template struct {
	Harness string `yaml:"harness"`
	Model   string `yaml:"model"`
	// Sandbox is the home's spec (ADR 0008), disk included. A command is
	// ignored: the home runs the agent loop.
	Sandbox task.Sandbox `yaml:"sandbox"`
	// Idle is a Go duration, 15m when empty.
	Idle string `yaml:"idle"`
	// Reads are channel patterns the agent watches besides its own: a
	// channel name, a glob on it, or * for every channel.
	Reads []string `yaml:"reads"`
	// Instructions are the agent's AGENT.md.
	Instructions string `yaml:"instructions"`
}

// IdleDuration is the template's idle, DefaultIdle when empty.
func (t Template) IdleDuration() (time.Duration, error) {
	if strings.TrimSpace(t.Idle) == "" {
		return DefaultIdle, nil
	}

	idle, err := time.ParseDuration(strings.TrimSpace(t.Idle))
	if err != nil || idle <= 0 {
		return 0, ez.New(ez.EINVALID, fmt.Sprintf("idle %q is not a positive duration such as 15m", t.Idle), err)
	}

	return idle, nil
}

// Validate checks what can be checked before a wake.
func (t Template) Validate() error {
	if t.Harness != "" {
		err := agent.Harness(t.Harness).Validate()
		if err != nil {
			return ez.New(ez.EINVALID, "harness "+t.Harness+" is not one of claude_code, codex_cli, pi", err)
		}
	}

	_, err := t.IdleDuration()
	if err != nil {
		return ez.Wrap(err)
	}

	for _, pattern := range t.Reads {
		err = validatePattern(pattern)
		if err != nil {
			return ez.Wrap(err)
		}
	}

	err = t.Sandbox.Validate()
	if err != nil {
		return ez.Wrap(err)
	}

	return nil
}

// ValidateRole checks a role names a template file.
func ValidateRole(role string) error {
	if !roleRegex.MatchString(role) {
		msg := fmt.Sprintf("role %q is invalid: use letters, digits, dashes and underscores, the name of a template in agents/", role)
		return ez.New(ez.EINVALID, msg, nil)
	}

	return nil
}

// TemplatePath is the template file of role: the role lowercased, so
// Reviewer reads reviewer.yaml.
func TemplatePath(dir string, role string) string {
	return filepath.Join(dir, strings.ToLower(role)+".yaml")
}

// LoadTemplate reads role's template from dir, on every call so edits show
// at once.
func LoadTemplate(dir string, role string) (*Template, error) {
	err := ValidateRole(role)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	path := TemplatePath(dir, role)

	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		msg := fmt.Sprintf("role %s has no template: write %s", role, path)
		return nil, ez.New(ez.EINVALID, msg, err)
	}
	if err != nil {
		return nil, ez.New(ez.EINTERNAL, "cannot read "+path, err)
	}

	tmpl := &Template{}

	err = yaml.Unmarshal(data, tmpl)
	if err != nil {
		return nil, ez.New(ez.EINVALID, "template "+path+" is not valid YAML: "+err.Error(), err)
	}

	err = tmpl.Validate()
	if err != nil {
		return nil, ez.New(ez.ErrorCode(err), "template "+path+": "+ez.ErrorMessage(err), err)
	}

	return tmpl, nil
}
