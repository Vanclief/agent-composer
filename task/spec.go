// Package task defines the Task spec: the YAML block that says what a Task
// VM boots from and what it may reach
// (docs/decisions/0003-tasks-run-in-microsandbox-vms.md).
package task

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"

	"github.com/vanclief/ez"
	"gopkg.in/yaml.v3"

	"github.com/vanclief/agent-composer/executor/sandbox"
	"github.com/vanclief/agent-composer/messaging"
	msgmodels "github.com/vanclief/agent-composer/models/messaging"
	"github.com/vanclief/agent-composer/secrets"
)

const (
	// LabelKey marks a sandbox as a Task. Its value is the Task name.
	LabelKey = "agc.task"
	// HomeDir is the sandbox user's home, where the repo is cloned.
	HomeDir = "/home/agent"
	// EnvTask and EnvMsgVsockPort are set by agc in every Task.
	EnvTask         = "AGC_TASK"
	EnvMsgVsockPort = "AGC_MSG_VSOCK_PORT"
)

var (
	envNameRe    = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
	volumeNameRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]*$`)
)

// Spec is a Task spec. Secrets are names only, never values.
type Spec struct {
	Image    string            `yaml:"image" json:"image"`
	CPUs     uint8             `yaml:"cpus" json:"cpus"`
	Memory   uint32            `yaml:"memory" json:"memory"` // MiB
	Repo     string            `yaml:"repo" json:"repo"`     // "github.com/owner/name" or a full https URL
	Ref      string            `yaml:"ref" json:"ref"`
	Setup    string            `yaml:"setup" json:"setup"`       // bash script run once after the clone
	Secrets  []string          `yaml:"secrets" json:"secrets"`   // "NAME" or "NAME@h1,h2"
	Env      map[string]string `yaml:"env" json:"env"`           // plain, readable by agents
	Volumes  []string          `yaml:"volumes" json:"volumes"`   // "NAME:DEST[:rw]" or "host:PATH:DEST[:rw]"
	Channels []string          `yaml:"channels" json:"channels"` // "#name" or "#name:read"
}

// file is a task.yaml: the spec at top level, with an optional name and
// title.
type file struct {
	Name  string `yaml:"name"`
	Title string `yaml:"title"`
	Spec  `yaml:",inline"`
}

// File is a parsed task.yaml.
type File struct {
	Name  string
	Title string
	Spec  Spec
}

// LoadFile reads a task.yaml.
func LoadFile(filePath string) (File, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return File{}, ez.New(ez.EINVALID, "cannot read task file "+filePath, err)
	}

	parsed, err := ParseFile(data)
	if err != nil {
		return File{}, ez.New(ez.EINVALID, "task file "+filePath+": "+ez.ErrorMessage(err), err)
	}

	return parsed, nil
}

// ParseFile parses a task.yaml. Unknown keys are rejected so a typo does
// not silently fall back to a default.
func ParseFile(data []byte) (File, error) {
	var f file
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	err := decoder.Decode(&f)
	if err != nil && !errors.Is(err, io.EOF) {
		return File{}, ez.New(ez.EINVALID, err.Error(), err)
	}

	return File{Name: strings.TrimSpace(f.Name), Title: strings.TrimSpace(f.Title), Spec: f.Spec}, nil
}

// Validate rejects entries the Task cannot honor and names the bad entry.
// Image, CPU and memory defaults and limits belong to sandbox.Spec.Validate.
func (s Spec) Validate() error {
	if s.Repo != "" {
		_, _, err := s.RepoURL()
		if err != nil {
			return ez.Wrap(err)
		}
	}

	if s.Ref != "" && s.Repo == "" {
		return ez.New(ez.EINVALID, "ref is set but repo is not", nil)
	}

	if strings.HasPrefix(s.Ref, "-") {
		return ez.New(ez.EINVALID, fmt.Sprintf("ref %q must not start with -", s.Ref), nil)
	}

	for _, entry := range s.Secrets {
		_, err := secrets.ParseRequest(entry)
		if err != nil {
			return ez.Wrap(err)
		}
	}

	for name := range s.Env {
		if !envNameRe.MatchString(name) {
			return ez.New(ez.EINVALID, fmt.Sprintf("env %q is not a valid variable name", name), nil)
		}

		if name == EnvTask || name == EnvMsgVsockPort {
			return ez.New(ez.EINVALID, fmt.Sprintf("env %s is set by agc and cannot be overridden", name), nil)
		}
	}

	for _, entry := range s.Volumes {
		_, err := ParseVolume(entry)
		if err != nil {
			return ez.Wrap(err)
		}
	}

	_, err := s.Grants()
	if err != nil {
		return ez.Wrap(err)
	}

	return nil
}

// RepoURL returns the clone URL and the directory the repo is cloned to.
// "github.com/o/n" becomes "https://github.com/o/n.git" in /home/agent/n.
// Only HTTPS is supported inside Tasks (ADR 0004).
func (s Spec) RepoURL() (string, string, error) {
	repo := strings.TrimSpace(s.Repo)
	cloneURL := repo
	if !strings.Contains(repo, "://") {
		cloneURL = "https://" + strings.TrimSuffix(repo, ".git") + ".git"
	}

	parsed, err := url.Parse(cloneURL)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil {
		msg := fmt.Sprintf("repo %q must be host/owner/name or an https URL without credentials", s.Repo)
		return "", "", ez.New(ez.EINVALID, msg, nil)
	}

	name := strings.TrimSuffix(path.Base(parsed.Path), ".git")
	if name == "" || name == "." || name == ".." || name == "/" {
		return "", "", ez.New(ez.EINVALID, fmt.Sprintf("repo %q has no repository name in its path", s.Repo), nil)
	}

	return cloneURL, path.Join(HomeDir, name), nil
}

// SandboxSpec is the sandbox the Task named name runs in. The guest reaches
// agc messaging on vsockPort, which the runtime connects to vsockPath.
func (s Spec) SandboxSpec(name string, bindings []secrets.Binding, vsockPath string, vsockPort uint32) (sandbox.Spec, error) {
	env := map[string]string{}
	for k, v := range s.Env {
		env[k] = v
	}
	env[EnvTask] = name
	env[EnvMsgVsockPort] = strconv.FormatUint(uint64(vsockPort), 10)

	volumes := []sandbox.Volume{}
	for _, entry := range s.Volumes {
		volume, err := ParseVolume(entry)
		if err != nil {
			return sandbox.Spec{}, ez.Wrap(err)
		}
		volumes = append(volumes, volume)
	}

	return sandbox.Spec{
		Name:      name,
		Image:     s.Image,
		CPUs:      s.CPUs,
		MemoryMiB: s.Memory,
		Env:       env,
		Secrets:   SandboxSecrets(bindings),
		Volumes:   volumes,
		VsockPath: vsockPath,
		VsockPort: vsockPort,
		Labels:    map[string]string{LabelKey: name},
	}, nil
}

// SandboxSecrets turns resolved bindings into the sandbox's secrets.
func SandboxSecrets(bindings []secrets.Binding) []sandbox.Secret {
	sandboxSecrets := []sandbox.Secret{}
	for _, binding := range bindings {
		sandboxSecrets = append(sandboxSecrets, sandbox.Secret{Name: binding.Name, Value: binding.Value, Hosts: binding.Hosts})
	}

	return sandboxSecrets
}

// SecretRequests parses the secrets entries.
func (s Spec) SecretRequests() ([]secrets.Request, error) {
	requests := []secrets.Request{}
	for _, entry := range s.Secrets {
		request, err := secrets.ParseRequest(entry)
		if err != nil {
			return nil, ez.Wrap(err)
		}
		requests = append(requests, request)
	}

	return requests, nil
}

// Grants turns the channels entries into messaging grants. "#name" may be
// read and posted to, "#name:read" only read.
func (s Spec) Grants() ([]messaging.Grant, error) {
	grants := []messaging.Grant{}
	for _, entry := range s.Channels {
		channel, suffix, hasSuffix := strings.Cut(strings.TrimSpace(entry), ":")
		if hasSuffix && suffix != "read" {
			msg := fmt.Sprintf("channel %q: the only suffix is :read", entry)
			return nil, ez.New(ez.EINVALID, msg, nil)
		}

		err := msgmodels.ValidateChannel(channel)
		if err != nil {
			return nil, ez.Wrap(err)
		}

		grants = append(grants, messaging.Grant{Channel: channel, ReadOnly: hasSuffix})
	}

	return grants, nil
}

// ParseVolume parses "NAME:DEST[:rw]" (a named volume) or
// "host:PATH:DEST[:rw]" (a host directory). Mounts are read-only unless
// rw is written.
func ParseVolume(entry string) (sandbox.Volume, error) {
	parts := strings.Split(strings.TrimSpace(entry), ":")
	volume := sandbox.Volume{}

	if parts[0] == "host" {
		volume.HostPath = true
		parts = parts[1:]
	}

	if len(parts) == 3 {
		if parts[2] != "rw" {
			return sandbox.Volume{}, ez.New(ez.EINVALID, fmt.Sprintf("volume %q: the only mode is rw, read-only is the default", entry), nil)
		}
		volume.ReadWrite = true
		parts = parts[:2]
	}

	if len(parts) != 2 {
		return sandbox.Volume{}, ez.New(ez.EINVALID, fmt.Sprintf("volume %q must be NAME:DEST[:rw] or host:PATH:DEST[:rw]", entry), nil)
	}

	volume.Source = parts[0]
	volume.Dest = parts[1]

	if volume.HostPath && !path.IsAbs(volume.Source) {
		return sandbox.Volume{}, ez.New(ez.EINVALID, fmt.Sprintf("volume %q: host path must be absolute", entry), nil)
	}

	if !volume.HostPath && !volumeNameRe.MatchString(volume.Source) {
		return sandbox.Volume{}, ez.New(ez.EINVALID, fmt.Sprintf("volume %q: name must be letters, digits, _, . or -", entry), nil)
	}

	if !path.IsAbs(volume.Dest) {
		return sandbox.Volume{}, ez.New(ez.EINVALID, fmt.Sprintf("volume %q: destination must be an absolute path", entry), nil)
	}

	return volume, nil
}
