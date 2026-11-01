// Package secrets reads the user's secrets file and turns the names a Task
// spec requests into host-bound bindings for the sandbox
// (docs/decisions/0004-secrets-are-placeholders-bound-to-hosts.md).
// Values never appear in errors, which name secrets by name only.
package secrets

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"

	"github.com/vanclief/ez"
)

// KnownHosts maps well-known secret names to their API hosts (ADR 0004 table).
var KnownHosts = map[string][]string{
	"ANTHROPIC_API_KEY":       {"api.anthropic.com"},
	"CLAUDE_CODE_OAUTH_TOKEN": {"api.anthropic.com"},
	"OPENAI_API_KEY":          {"api.openai.com"},
	"GITHUB_TOKEN":            {"api.github.com", "github.com"},
}

var (
	nameRe = regexp.MustCompile(`^[A-Z][A-Z0-9_]*$`)
	hostRe = regexp.MustCompile(`^(\*\.)?[a-z0-9]([a-z0-9-]*[a-z0-9])?(\.[a-z0-9]([a-z0-9-]*[a-z0-9])?)*$`)
)

// DefaultPath returns the secrets file path for this agc home: the
// AGENT_COMPOSER_HOME directory, or ~/.agent_composer. The same rule as
// workflow.ResolveHomeDir, repeated here so this package stays free of the
// workflow dependency tree.
func DefaultPath() (string, error) {
	home := strings.TrimSpace(os.Getenv("AGENT_COMPOSER_HOME"))
	if home == "" {
		userHome, err := os.UserHomeDir()
		if err != nil {
			return "", ez.Wrap(err)
		}
		home = filepath.Join(userHome, ".agent_composer")
	}

	return filepath.Join(home, "config", "secrets.env"), nil
}

// Store holds the secret values read from one file.
type Store struct {
	path   string
	values map[string]string
}

// Load reads the file at path. A missing file yields an empty Store. The
// file must be owner-only (0600 or stricter). The mode check is skipped on
// windows.
func Load(path string) (*Store, error) {
	store := &Store{path: path, values: map[string]string{}}

	info, err := os.Stat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return store, nil
	}
	if err != nil {
		return nil, ez.New(ez.EINTERNAL, "cannot read secrets file "+path, err)
	}

	perm := info.Mode().Perm()
	if runtime.GOOS != "windows" && perm&0o077 != 0 {
		msg := fmt.Sprintf("secrets file %s has mode %04o, it must be readable by its owner only: run chmod 600 %s", path, uint32(perm), path)
		return nil, ez.New(ez.ENOTAUTHORIZED, msg, nil)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, ez.New(ez.EINTERNAL, "cannot read secrets file "+path, err)
	}

	scanner := bufio.NewScanner(bytes.NewReader(data))
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		line = strings.TrimSpace(strings.TrimPrefix(line, "export "))
		name, value, found := strings.Cut(line, "=")
		name = strings.TrimSpace(name)
		if !found || !nameRe.MatchString(name) {
			msg := fmt.Sprintf("secrets file %s line %d: expected NAME=value with NAME matching [A-Z][A-Z0-9_]*", path, lineNo)
			return nil, ez.New(ez.EINVALID, msg, nil)
		}

		store.values[name] = unquote(strings.TrimSpace(value))
	}

	err = scanner.Err()
	if err != nil {
		return nil, ez.New(ez.EINTERNAL, "cannot read secrets file "+path, err)
	}

	return store, nil
}

// unquote strips one surrounding pair of matching single or double quotes.
func unquote(value string) string {
	if len(value) < 2 {
		return value
	}

	first := value[0]
	last := value[len(value)-1]
	if first == last && (first == '"' || first == '\'') {
		return value[1 : len(value)-1]
	}

	return value
}

// Names returns the configured secret names, sorted.
func (s *Store) Names() []string {
	names := make([]string, 0, len(s.values))
	for name := range s.values {
		names = append(names, name)
	}
	sort.Strings(names)

	return names
}

// Request is one entry from a Task spec `secrets:` list.
type Request struct {
	Name  string
	Hosts []string
}

// ParseRequest parses "NAME" or "NAME@host1,host2". Hosts are lowercase
// hostnames with an optional leading "*." wildcard, and no scheme, path or
// port.
func ParseRequest(s string) (Request, error) {
	name, hostList, hasHosts := strings.Cut(strings.TrimSpace(s), "@")
	if !nameRe.MatchString(name) {
		msg := fmt.Sprintf("secret name %q must match [A-Z][A-Z0-9_]*", name)
		return Request{}, ez.New(ez.EINVALID, msg, nil)
	}

	req := Request{Name: name}
	if !hasHosts {
		return req, nil
	}

	for _, host := range strings.Split(hostList, ",") {
		host = strings.TrimSpace(host)
		if !hostRe.MatchString(host) {
			msg := fmt.Sprintf("secret %s: host %q must be a lowercase hostname, optionally starting with *., without scheme, path or port", name, host)
			return Request{}, ez.New(ez.EINVALID, msg, nil)
		}
		req.Hosts = append(req.Hosts, host)
	}

	return req, nil
}

// Binding is what the sandbox receives: name, value, allowed hosts.
type Binding struct {
	Name  string
	Value string
	Hosts []string
}

// Resolve returns one Binding per request. Hosts come from the request when
// given, otherwise from KnownHosts. Every problem is reported in one error and
// its code is the code of the first problem found.
func (s *Store) Resolve(requests []Request) ([]Binding, error) {
	bindings := make([]Binding, 0, len(requests))
	problems := []string{}
	code := ""

	for _, req := range requests {
		value, ok := s.values[req.Name]
		if !ok {
			problems = append(problems, fmt.Sprintf("secret %s is not set in %s", req.Name, s.path))
			if code == "" {
				code = ez.ENOTFOUND
			}
		}

		hosts := req.Hosts
		if len(hosts) == 0 {
			hosts = KnownHosts[req.Name]
		}
		if len(hosts) == 0 {
			problems = append(problems, fmt.Sprintf("secret %s has no known hosts: write %s@host in the spec", req.Name, req.Name))
			if code == "" {
				code = ez.EINVALID
			}
		}

		bindings = append(bindings, Binding{Name: req.Name, Value: value, Hosts: append([]string(nil), hosts...)})
	}

	if len(problems) > 0 {
		return nil, ez.New(code, strings.Join(problems, "\n"), nil)
	}

	return bindings, nil
}
