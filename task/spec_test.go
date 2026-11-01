package task

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/vanclief/ez"

	"github.com/vanclief/agent-composer/executor/sandbox"
	"github.com/vanclief/agent-composer/messaging"
	"github.com/vanclief/agent-composer/secrets"
)

func writeFile(t *testing.T, content string) string {
	t.Helper()

	filePath := filepath.Join(t.TempDir(), "task.yaml")
	err := os.WriteFile(filePath, []byte(content), 0o600)
	if err != nil {
		t.Fatal(err)
	}

	return filePath
}

func TestLoadFileWithName(t *testing.T) {
	filePath := writeFile(t, `
name: review-42
title: Review PR 42
image: agc-agent
cpus: 2
memory: 4096
repo: github.com/vanclief/agent-composer
ref: master
setup: |
  go mod download
secrets: [CLAUDE_CODE_OAUTH_TOKEN, GITHUB_TOKEN]
env:
  GOFLAGS: -mod=mod
volumes:
  - go-mod-cache:/home/agent/go/pkg/mod:rw
channels: ["#releases", "#alerts:read"]
`)

	loaded, err := LoadFile(filePath)
	if err != nil {
		t.Fatal(err)
	}
	spec := loaded.Spec

	if loaded.Name != "review-42" || loaded.Title != "Review PR 42" {
		t.Fatalf("name = %q, title = %q", loaded.Name, loaded.Title)
	}

	want := Spec{
		Image:    "agc-agent",
		CPUs:     2,
		Memory:   4096,
		Repo:     "github.com/vanclief/agent-composer",
		Ref:      "master",
		Setup:    "go mod download\n",
		Secrets:  []string{"CLAUDE_CODE_OAUTH_TOKEN", "GITHUB_TOKEN"},
		Env:      map[string]string{"GOFLAGS": "-mod=mod"},
		Volumes:  []string{"go-mod-cache:/home/agent/go/pkg/mod:rw"},
		Channels: []string{"#releases", "#alerts:read"},
	}
	if !reflect.DeepEqual(spec, want) {
		t.Fatalf("spec = %+v, want %+v", spec, want)
	}

	err = spec.Validate()
	if err != nil {
		t.Fatal(err)
	}
}

func TestLoadFileBareSpec(t *testing.T) {
	loaded, err := LoadFile(writeFile(t, "image: debian\nmemory: 512\n"))
	if err != nil {
		t.Fatal(err)
	}

	if loaded.Name != "" || loaded.Title != "" || loaded.Spec.Image != "debian" || loaded.Spec.Memory != 512 {
		t.Fatalf("got %+v", loaded)
	}
}

func TestLoadFileRejectsUnknownKey(t *testing.T) {
	_, err := LoadFile(writeFile(t, "image: debian\ncpu: 2\n"))
	if err == nil || ez.ErrorCode(err) != ez.EINVALID || !strings.Contains(ez.ErrorMessage(err), "cpu") {
		t.Fatalf("expected EINVALID naming cpu, got %v", err)
	}
}

func TestParseFile(t *testing.T) {
	parsed, err := ParseFile([]byte("title: '  Review ads  '\nrepo: github.com/o/n\n"))
	if err != nil {
		t.Fatal(err)
	}

	if parsed.Title != "Review ads" || parsed.Name != "" || parsed.Spec.Repo != "github.com/o/n" {
		t.Fatalf("got %+v", parsed)
	}

	parsed, err = ParseFile(nil)
	if err != nil || !reflect.DeepEqual(parsed, File{}) {
		t.Fatalf("empty file: got %+v, err %v", parsed, err)
	}

	_, err = ParseFile([]byte("repo: [unclosed\n"))
	if ez.ErrorCode(err) != ez.EINVALID {
		t.Fatalf("expected EINVALID for bad YAML, got %v", err)
	}
}

func TestParseVolume(t *testing.T) {
	cases := []struct {
		entry string
		want  sandbox.Volume
	}{
		{"datasets:/data", sandbox.Volume{Source: "datasets", Dest: "/data"}},
		{"go-mod-cache:/home/agent/go/pkg/mod:rw", sandbox.Volume{Source: "go-mod-cache", Dest: "/home/agent/go/pkg/mod", ReadWrite: true}},
		{"host:/srv/models:/models", sandbox.Volume{Source: "/srv/models", Dest: "/models", HostPath: true}},
		{"host:/srv/out:/out:rw", sandbox.Volume{Source: "/srv/out", Dest: "/out", HostPath: true, ReadWrite: true}},
	}

	for _, tc := range cases {
		got, err := ParseVolume(tc.entry)
		if err != nil {
			t.Fatalf("%s: %v", tc.entry, err)
		}

		if got != tc.want {
			t.Fatalf("%s: got %+v, want %+v", tc.entry, got, tc.want)
		}
	}
}

func TestParseVolumeRejects(t *testing.T) {
	entries := []string{
		"datasets",
		"datasets:/data:ro",
		"datasets:data",
		"host:/srv/models",
		"host:srv:/models",
		"bad/name:/data",
		"a:/b:rw:extra",
	}

	for _, entry := range entries {
		_, err := ParseVolume(entry)
		if err == nil {
			t.Fatalf("%s: expected an error", entry)
		}

		if ez.ErrorCode(err) != ez.EINVALID || !strings.Contains(ez.ErrorMessage(err), entry) {
			t.Fatalf("%s: expected EINVALID naming the entry, got %v", entry, err)
		}
	}
}

func TestGrants(t *testing.T) {
	spec := Spec{Channels: []string{"#releases", "#alerts:read"}}

	grants, err := spec.Grants()
	if err != nil {
		t.Fatal(err)
	}

	want := []messaging.Grant{{Channel: "#releases"}, {Channel: "#alerts", ReadOnly: true}}
	if !reflect.DeepEqual(grants, want) {
		t.Fatalf("grants = %+v", grants)
	}

	for _, entry := range []string{"releases", "#alerts:write", "#Bad"} {
		_, err = Spec{Channels: []string{entry}}.Grants()
		if err == nil || ez.ErrorCode(err) != ez.EINVALID {
			t.Fatalf("%s: expected EINVALID, got %v", entry, err)
		}
	}
}

func TestRepoURL(t *testing.T) {
	cases := []struct {
		repo string
		url  string
		dir  string
	}{
		{"github.com/vanclief/agent-composer", "https://github.com/vanclief/agent-composer.git", "/home/agent/agent-composer"},
		{"github.com/o/n.git", "https://github.com/o/n.git", "/home/agent/n"},
		{"https://gitlab.com/group/sub/proj.git", "https://gitlab.com/group/sub/proj.git", "/home/agent/proj"},
		{"https://github.com/o/n", "https://github.com/o/n", "/home/agent/n"},
	}

	for _, tc := range cases {
		cloneURL, dir, err := Spec{Repo: tc.repo}.RepoURL()
		if err != nil {
			t.Fatalf("%s: %v", tc.repo, err)
		}

		if cloneURL != tc.url || dir != tc.dir {
			t.Fatalf("%s: got %s %s", tc.repo, cloneURL, dir)
		}
	}

	for _, repo := range []string{"git@github.com:o/n.git", "ssh://github.com/o/n", "http://github.com/o/n", "https://user:pw@github.com/o/n", "https://github.com/"} {
		_, _, err := Spec{Repo: repo}.RepoURL()
		if err == nil || ez.ErrorCode(err) != ez.EINVALID {
			t.Fatalf("%s: expected EINVALID, got %v", repo, err)
		}
	}
}

func TestValidateRejects(t *testing.T) {
	cases := []struct {
		spec Spec
		name string
	}{
		{Spec{Secrets: []string{"lower"}}, "lower"},
		{Spec{Secrets: []string{"API_KEY@Bad.Host"}}, "Bad.Host"},
		{Spec{Env: map[string]string{"AGC_TASK": "x"}}, "AGC_TASK"},
		{Spec{Env: map[string]string{"A-B": "x"}}, "A-B"},
		{Spec{Volumes: []string{"cache"}}, "cache"},
		{Spec{Channels: []string{"#a:write"}}, "#a:write"},
		{Spec{Repo: "git@github.com:o/n.git"}, "git@github.com"},
		{Spec{Ref: "main"}, "ref"},
	}

	for _, tc := range cases {
		err := tc.spec.Validate()
		if err == nil || ez.ErrorCode(err) != ez.EINVALID || !strings.Contains(ez.ErrorMessage(err), tc.name) {
			t.Fatalf("%+v: expected EINVALID naming %s, got %v", tc.spec, tc.name, err)
		}
	}
}

func TestSandboxSpec(t *testing.T) {
	spec := Spec{
		Image:   "agc-agent",
		CPUs:    2,
		Memory:  1024,
		Env:     map[string]string{"GOFLAGS": "-mod=mod"},
		Volumes: []string{"cache:/cache:rw"},
	}
	bindings := []secrets.Binding{{Name: "GITHUB_TOKEN", Value: "v", Hosts: []string{"github.com"}}}

	got, err := spec.SandboxSpec("review-42", bindings, "/tmp/review-42.sock", 5100)
	if err != nil {
		t.Fatal(err)
	}

	want := sandbox.Spec{
		Name:      "review-42",
		Image:     "agc-agent",
		CPUs:      2,
		MemoryMiB: 1024,
		Env:       map[string]string{"GOFLAGS": "-mod=mod", "AGC_TASK": "review-42", "AGC_MSG_VSOCK_PORT": "5100"},
		Secrets:   []sandbox.Secret{{Name: "GITHUB_TOKEN", Value: "v", Hosts: []string{"github.com"}}},
		Volumes:   []sandbox.Volume{{Source: "cache", Dest: "/cache", ReadWrite: true}},
		VsockPath: "/tmp/review-42.sock",
		VsockPort: 5100,
		Labels:    map[string]string{"agc.task": "review-42"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v\nwant %+v", got, want)
	}

	if len(spec.Env) != 1 {
		t.Fatalf("SandboxSpec changed the spec env: %v", spec.Env)
	}
}
