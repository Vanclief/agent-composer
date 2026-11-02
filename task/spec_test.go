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
brief: |
  Review PR 42 and post findings.
accept:
  - 'the findings are posted in #review-42'
  - go test passes
assignee: claude_code
model: ' claude-opus-5-5 '
manager: orchestrator
owner: franco
for: review-prs
sandbox:
  image: agc-agent
  cpus: 2
  memory: 4096
  disk: 10G
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
  command: claude --dangerously-skip-permissions
`)

	loaded, err := LoadFile(filePath)
	if err != nil {
		t.Fatal(err)
	}

	if loaded.Name != "review-42" || loaded.Title != "Review PR 42" || loaded.Assignee != "claude_code" || loaded.For != "review-prs" {
		t.Fatalf("got %+v", loaded)
	}
	if loaded.Model != "claude-opus-5-5" || loaded.Manager != "orchestrator" || loaded.Owner != "franco" {
		t.Fatalf("model, manager, owner = %q, %q, %q", loaded.Model, loaded.Manager, loaded.Owner)
	}
	if len(loaded.Accept) != 2 || loaded.Accept[0] != "the findings are posted in #review-42" || loaded.Accept[1] != "go test passes" {
		t.Fatalf("accept = %q", loaded.Accept)
	}
	if loaded.Brief != "Review PR 42 and post findings.\n" {
		t.Fatalf("brief = %q", loaded.Brief)
	}

	want := Sandbox{
		Spec: Spec{
			Image:    "agc-agent",
			CPUs:     2,
			Memory:   4096,
			Disk:     "10G",
			Repo:     "github.com/vanclief/agent-composer",
			Ref:      "master",
			Setup:    "go mod download\n",
			Secrets:  []string{"CLAUDE_CODE_OAUTH_TOKEN", "GITHUB_TOKEN"},
			Env:      map[string]string{"GOFLAGS": "-mod=mod"},
			Volumes:  []string{"go-mod-cache:/home/agent/go/pkg/mod:rw"},
			Channels: []string{"#releases", "#alerts:read"},
		},
		Command: "claude --dangerously-skip-permissions",
	}
	if loaded.Sandbox == nil || !reflect.DeepEqual(*loaded.Sandbox, want) {
		t.Fatalf("sandbox = %+v, want %+v", loaded.Sandbox, want)
	}

	err = loaded.Sandbox.Validate()
	if err != nil {
		t.Fatal(err)
	}
}

func TestLoadFileWithoutSandbox(t *testing.T) {
	loaded, err := LoadFile(writeFile(t, "title: Call the bank\nbrief: Ask about the fee.\nassignee: Franco\n"))
	if err != nil {
		t.Fatal(err)
	}

	if loaded.Title != "Call the bank" || loaded.Assignee != "Franco" || loaded.Sandbox != nil {
		t.Fatalf("got %+v", loaded)
	}
}

func TestLoadFileRejectsUnknownKey(t *testing.T) {
	_, err := LoadFile(writeFile(t, "sandbox:\n  image: debian\n  cpu: 2\n"))
	if err == nil || ez.ErrorCode(err) != ez.EINVALID || !strings.Contains(ez.ErrorMessage(err), "cpu") {
		t.Fatalf("expected EINVALID naming cpu, got %v", err)
	}

	// The environment lives under sandbox:, not at top level.
	_, err = LoadFile(writeFile(t, "title: x\nrepo: github.com/o/n\n"))
	if ez.ErrorCode(err) != ez.EINVALID || !strings.Contains(ez.ErrorMessage(err), "repo") {
		t.Fatalf("expected EINVALID naming repo, got %v", err)
	}
}

func TestParseFile(t *testing.T) {
	parsed, err := ParseFile([]byte("title: '  Review ads  '\nsandbox:\n  repo: github.com/o/n\n"))
	if err != nil {
		t.Fatal(err)
	}

	if parsed.Title != "Review ads" || parsed.Name != "" || parsed.Sandbox.Repo != "github.com/o/n" {
		t.Fatalf("got %+v", parsed)
	}

	if parsed.Notify != nil {
		t.Fatalf("notify without notify: %v", *parsed.Notify)
	}

	parsed, err = ParseFile([]byte("title: Review ads\nnotify: false\n"))
	if err != nil || parsed.Notify == nil || *parsed.Notify {
		t.Fatalf("notify: false: got %+v, err %v", parsed, err)
	}

	parsed, err = ParseFile(nil)
	if err != nil || !reflect.DeepEqual(parsed, File{}) {
		t.Fatalf("empty file: got %+v, err %v", parsed, err)
	}

	_, err = ParseFile([]byte("sandbox: [unclosed\n"))
	if ez.ErrorCode(err) != ez.EINVALID {
		t.Fatalf("expected EINVALID for bad YAML, got %v", err)
	}
}

func TestParseSandbox(t *testing.T) {
	parsed, err := ParseSandbox([]byte("image: debian\nmemory: 512\ncommand: bash\n"))
	if err != nil {
		t.Fatal(err)
	}

	if parsed.Image != "debian" || parsed.Memory != 512 || parsed.Command != "bash" {
		t.Fatalf("got %+v", parsed)
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
		{Spec{Secrets: []string{"ANTHROPIC_WORK as claude-token"}}, `"ANTHROPIC_WORK as claude-token"`},
		{Spec{Secrets: []string{"ANTHROPIC_WORK as CLAUDE_CODE_OAUTH_TOKEN", "ANTHROPIC_HOME as CLAUDE_CODE_OAUTH_TOKEN"}}, `"ANTHROPIC_HOME as CLAUDE_CODE_OAUTH_TOKEN"`},
		{Spec{Secrets: []string{"CLAUDE_CODE_OAUTH_TOKEN", "ANTHROPIC_WORK as CLAUDE_CODE_OAUTH_TOKEN"}}, `"ANTHROPIC_WORK as CLAUDE_CODE_OAUTH_TOKEN"`},
		{Spec{Secrets: []string{"ANTHROPIC_WORK as CLAUDE_CODE_OAUTH_TOKEN", "CLAUDE_CODE_OAUTH_TOKEN"}}, `"CLAUDE_CODE_OAUTH_TOKEN"`},
		{Spec{Env: map[string]string{"AGC_TASK": "x"}}, "AGC_TASK"},
		{Spec{Env: map[string]string{"A-B": "x"}}, "A-B"},
		{Spec{Volumes: []string{"cache"}}, "cache"},
		{Spec{Channels: []string{"#a:write"}}, "#a:write"},
		{Spec{Repo: "git@github.com:o/n.git"}, "git@github.com"},
		{Spec{Ref: "main"}, "ref"},
		{Spec{Disk: "1G"}, "minimum is 2G"},
		{Spec{Disk: "1500M"}, "minimum is 2G"},
		{Spec{Disk: "10GB"}, "10GB"},
		{Spec{Disk: "-5G"}, "-5G"},
		{Spec{Disk: "lots"}, "lots"},
		{Spec{Disk: "99999T"}, "too large"},
		{Spec{Volumes: []string{"x:/home/agent:rw"}}, "mounted by agc"},
		{Spec{Volumes: []string{"x:/home/agent/.harness/claude:rw"}}, "mounted by agc"},
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

	got, err := spec.SandboxSpec("review-42", "review-42-2", bindings, "/tmp/review-42-2.sock", 5100)
	if err != nil {
		t.Fatal(err)
	}

	want := sandbox.Spec{
		Name:      "review-42-2",
		Image:     "agc-agent",
		CPUs:      2,
		MemoryMiB: 1024,
		Env:       map[string]string{"GOFLAGS": "-mod=mod", "AGC_TASK": "review-42", "AGC_SANDBOX": "review-42-2", "AGC_MSG_VSOCK_PORT": "5100"},
		Secrets:   []sandbox.Secret{{Name: "GITHUB_TOKEN", Value: "v", Hosts: []string{"github.com"}}},
		Volumes: []sandbox.Volume{
			{Source: "cache", Dest: "/cache", ReadWrite: true},
			{Source: "agc-review-42-2", Dest: "/home/agent/.harness", ReadWrite: true, QuotaMiB: 4096},
		},
		DiskDest:  "/home/agent",
		DiskMiB:   0,
		VsockPath: "/tmp/review-42-2.sock",
		VsockPort: 5100,
		Labels:    map[string]string{"agc.task": "review-42", "agc.sandbox": "review-42-2"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v\nwant %+v", got, want)
	}

	if len(spec.Env) != 1 {
		t.Fatalf("SandboxSpec changed the spec env: %v", spec.Env)
	}

	spec.Disk = "50g"
	got, err = spec.SandboxSpec("review-42", "review-42-2", bindings, "/tmp/review-42-2.sock", 5100)
	if err != nil {
		t.Fatal(err)
	}
	if got.DiskMiB != 50*1024 {
		t.Fatalf("disk 50g: DiskMiB = %d", got.DiskMiB)
	}
}

func TestDiskMiB(t *testing.T) {
	cases := map[string]uint32{"": 0, "2G": 2048, "2048M": 2048, " 20g ": 20480, "1T": 1024 * 1024}
	for disk, want := range cases {
		got, err := Spec{Disk: disk}.DiskMiB()
		if err != nil || got != want {
			t.Fatalf("disk %q: got %d, %v, want %d", disk, got, err, want)
		}
	}
}

// An account: a named secret handed to the sandbox under the variable the
// harness reads (ADR 0004 "Accounts"), from validation to the sandbox spec.
func TestSandboxSpecAccount(t *testing.T) {
	path := filepath.Join(t.TempDir(), "secrets.env")
	err := os.WriteFile(path, []byte("ANTHROPIC_WORK=work-account-value\nGITHUB_TOKEN=github-value\n"), 0o600)
	if err != nil {
		t.Fatal(err)
	}
	store, err := secrets.Load(path)
	if err != nil {
		t.Fatal(err)
	}

	spec := Spec{Image: "agc-agent", Secrets: []string{"ANTHROPIC_WORK as CLAUDE_CODE_OAUTH_TOKEN", "GITHUB_TOKEN"}}
	err = spec.Validate()
	if err != nil {
		t.Fatal(err)
	}

	requests, err := spec.SecretRequests()
	if err != nil {
		t.Fatal(err)
	}
	bindings, err := store.Resolve(requests)
	if err != nil {
		t.Fatal(err)
	}

	got, err := spec.SandboxSpec("work", "work-1", bindings, "/tmp/work-1.sock", 5100)
	if err != nil {
		t.Fatal(err)
	}

	want := []sandbox.Secret{
		{Name: "CLAUDE_CODE_OAUTH_TOKEN", Value: "work-account-value", Hosts: []string{"api.anthropic.com"}},
		{Name: "GITHUB_TOKEN", Value: "github-value", Hosts: []string{"api.github.com", "github.com"}},
	}
	if !reflect.DeepEqual(got.Secrets, want) {
		t.Fatalf("secrets = %+v, want %+v", got.Secrets, want)
	}
	_, set := got.Env["ANTHROPIC_WORK"]
	if set {
		t.Fatal("the account name leaked into the plain env")
	}
}

// Before accounts a plain NAME could be listed twice; that still validates,
// so specs written then are not broken.
func TestValidateKeepsPlainDuplicates(t *testing.T) {
	spec := Spec{Secrets: []string{"GITHUB_TOKEN", "GITHUB_TOKEN@ghe.example.com"}}
	err := spec.Validate()
	if err != nil {
		t.Fatalf("plain duplicate rejected: %v", err)
	}
}
