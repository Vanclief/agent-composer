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

// PeerToken is the secret agc servers share to call each other (ADR 0018).
// agc reads it from its own secrets file and checks it on the peer routes;
// it is never sent to a third party and never given to a sandbox.
const PeerToken = "AGC_PEER_TOKEN"

// KnownHosts maps well-known secret names to their API hosts (ADR 0004
// table). An entry with no hosts is agc's own secret: it is bound to no
// host, so no spec can request it, with or without NAME@host.
var KnownHosts = map[string][]string{
	"ANTHROPIC_API_KEY":       {"api.anthropic.com"},
	"CLAUDE_CODE_OAUTH_TOKEN": {"api.anthropic.com"},
	"OPENAI_API_KEY":          {"api.openai.com"},
	"GITHUB_TOKEN":            {"api.github.com", "github.com"},
	PeerToken:                 {},
}

// agcOnly reports whether name is agc's own secret, listed in KnownHosts
// bound to no host.
func agcOnly(name string) bool {
	hosts, ok := KnownHosts[name]
	return ok && len(hosts) == 0
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

// Lookup returns the value of the secret called name, for a process agc
// runs on this host (a schedule's poll command). Callers never print it.
func (s *Store) Lookup(name string) (string, bool) {
	value, ok := s.values[name]
	return value, ok
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
		if !ok && !agcOnly(req.Name) {
			problems = append(problems, fmt.Sprintf("secret %s is not set in %s", req.Name, s.path))
			if code == "" {
				code = ez.ENOTFOUND
			}
		}

		hosts, problem := hostsFor(req)
		if problem != "" {
			problems = append(problems, problem)
			if code == "" {
				code = ez.EINVALID
			}
		}

		bindings = append(bindings, Binding{Name: req.Name, Value: value, Hosts: hosts})
	}

	if len(problems) > 0 {
		return nil, ez.New(code, strings.Join(problems, "\n"), nil)
	}

	return bindings, nil
}

// Bind is Resolve without values, for a sandbox on another agc server,
// which reads the values from its own secrets file (ADR 0018): one
// Binding per request with its hosts and an empty Value.
func Bind(requests []Request) ([]Binding, error) {
	bindings := make([]Binding, 0, len(requests))
	problems := []string{}

	for _, req := range requests {
		hosts, problem := hostsFor(req)
		if problem != "" {
			problems = append(problems, problem)
		}

		bindings = append(bindings, Binding{Name: req.Name, Hosts: hosts})
	}

	if len(problems) > 0 {
		return nil, ez.New(ez.EINVALID, strings.Join(problems, "\n"), nil)
	}

	return bindings, nil
}

// Fill sets each binding's value from the store, on the server that runs
// the sandbox (ADR 0018). where names that server in the error, which
// lists every missing secret by name.
func (s *Store) Fill(bindings []Binding, where string) ([]Binding, error) {
	filled := make([]Binding, 0, len(bindings))
	problems := []string{}
	code := ""

	for _, binding := range bindings {
		if agcOnly(binding.Name) || len(binding.Hosts) == 0 {
			problems = append(problems, fmt.Sprintf("secret %s is bound to no host and is never given to a sandbox", binding.Name))
			if code == "" {
				code = ez.EINVALID
			}
			continue
		}

		value, ok := s.values[binding.Name]
		if !ok {
			problems = append(problems, fmt.Sprintf("secret %s is not set on server %s: add it to %s there", binding.Name, where, s.path))
			if code == "" {
				code = ez.ENOTFOUND
			}
			continue
		}

		filled = append(filled, Binding{Name: binding.Name, Value: value, Hosts: append([]string(nil), binding.Hosts...)})
	}

	if len(problems) > 0 {
		return nil, ez.New(code, strings.Join(problems, "\n"), nil)
	}

	return filled, nil
}

// hostsFor is the request's hosts, or KnownHosts', or a problem: none
// known, or the name is agc's own secret.
func hostsFor(req Request) ([]string, string) {
	if agcOnly(req.Name) {
		return nil, fmt.Sprintf("secret %s is agc's own, bound to no host, and is never given to a sandbox", req.Name)
	}

	hosts := req.Hosts
	if len(hosts) == 0 {
		hosts = KnownHosts[req.Name]
	}
	if len(hosts) == 0 {
		return nil, fmt.Sprintf("secret %s has no known hosts: write %s@host in the spec", req.Name, req.Name)
	}

	return append([]string(nil), hosts...), ""
}
