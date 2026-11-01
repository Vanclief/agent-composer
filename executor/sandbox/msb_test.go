package sandbox

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/vanclief/ez"
)

func TestCreateArgs(t *testing.T) {
	spec := Spec{
		Name:      "task-1",
		Image:     "agc-agent:dev",
		CPUs:      2,
		MemoryMiB: 4096,
		User:      "agent",
		Env:       map[string]string{"GOFLAGS": "-mod=mod", "A": "1"},
		Labels:    map[string]string{"agc.task": "t1"},
		Secrets: []Secret{
			{Name: "GITHUB_TOKEN", Value: "do-not-leak", Hosts: []string{"api.github.com", "github.com"}},
		},
		Volumes: []Volume{
			{Source: "go-mod-cache", Dest: "/home/agent/go/pkg/mod", ReadWrite: true},
			{Source: "datasets", Dest: "/data"},
			{Source: "/srv/models", Dest: "/models", HostPath: true},
		},
		VsockPath: "/run/agc/t1.sock",
		VsockPort: 5000,
	}
	want := []string{
		"create", "agc-agent:dev",
		"--name", "task-1",
		"-c", "2",
		"-m", "4096M",
		"-u", "agent",
		"-e", "A=1",
		"-e", "GOFLAGS=-mod=mod",
		"--label", "agc.task=t1",
		"--secret", "GITHUB_TOKEN@api.github.com,github.com",
		"--mount-named", "go-mod-cache:/home/agent/go/pkg/mod",
		"--mount-named", "datasets:/data:ro",
		"--mount-dir", "/srv/models:/models:ro",
		"--vsock", "/run/agc/t1.sock:5000",
	}
	got := createArgs(spec)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("createArgs =\n%q\nwant\n%q", got, want)
	}
	for _, arg := range got {
		if strings.Contains(arg, "do-not-leak") {
			t.Errorf("argv contains a secret value: %q", arg)
		}
	}
}

func TestMsbPathMissing(t *testing.T) {
	t.Setenv("MSB_PATH", "")
	t.Setenv("PATH", "")
	t.Setenv("HOME", t.TempDir())
	_, err := msbPath()
	if ez.ErrorCode(err) != ez.EUNAVAILABLE {
		t.Fatalf("msbPath: %v, want EUNAVAILABLE", err)
	}
	if !strings.Contains(ez.ErrorMessage(err), ".microsandbox/bin/msb") {
		t.Errorf("message %q does not name the paths tried", ez.ErrorMessage(err))
	}
}

// TestRunMsb uses a fake msb that prints a secret from its environment to
// stderr and fails, to check that secrets reach the child only and that
// stderr becomes the error message.
func TestRunMsb(t *testing.T) {
	fake := filepath.Join(t.TempDir(), "msb")
	script := "#!/bin/sh\necho \"  $1: secret is $AGC_FAKE_SECRET  \" >&2\nexit 1\n"
	err := os.WriteFile(fake, []byte(script), 0o755)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("MSB_PATH", fake)

	secrets := []Secret{{Name: "AGC_FAKE_SECRET", Value: "child-only", Hosts: []string{"example.com"}}}
	err = runMsb(context.Background(), secrets, "start", "task-1")
	if ez.ErrorCode(err) != ez.EINTERNAL {
		t.Fatalf("runMsb: %v, want EINTERNAL", err)
	}
	if ez.ErrorMessage(err) != "start: secret is child-only" {
		t.Errorf("message = %q", ez.ErrorMessage(err))
	}
	if os.Getenv("AGC_FAKE_SECRET") != "" {
		t.Error("secret leaked into this process's environment")
	}
}
