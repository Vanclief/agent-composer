//go:build cgo

package sandbox

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/vanclief/agent-composer/executor"
	"github.com/vanclief/ez"
)

// The tests in this file run against a real microsandbox runtime. They need
// a host with KVM (or Apple Silicon), msb installed, and the agent image
// loaded with msb.
func integrationSetup(t *testing.T) (string, string) {
	if os.Getenv("AGC_SANDBOX_TEST") != "1" {
		t.Skip("set AGC_SANDBOX_TEST=1 to run against a microsandbox runtime")
	}
	image := os.Getenv("AGC_SANDBOX_IMAGE")
	if image == "" {
		image = "agc-agent:spike"
	}
	suffix := make([]byte, 4)
	_, err := rand.Read(suffix)
	if err != nil {
		t.Fatal(err)
	}
	return "agc-test-" + hex.EncodeToString(suffix), image
}

// removeOnCleanup releases *exe, then stops and removes the sandbox.
func removeOnCleanup(t *testing.T, name string, exe **Executor) {
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		if *exe != nil {
			(*exe).Detach(ctx)
		}
		err := Stop(ctx, name)
		if err != nil {
			t.Logf("Stop: %v", err)
		}
		err = Remove(ctx, name)
		if err != nil {
			t.Errorf("Remove: %v", err)
		}
	})
}

func TestSandbox(t *testing.T) {
	name, image := integrationSetup(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	var exe *Executor
	removeOnCleanup(t, name, &exe)
	exe, err := Create(ctx, Spec{Name: name, Image: image, MemoryMiB: 1024, Labels: map[string]string{"agc.test": name}})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	status, err := Status(ctx, name)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if status != "running" {
		t.Errorf("Status = %q, want running", status)
	}

	infos, err := List(ctx, "agc.test")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	found := false
	for _, info := range infos {
		if info.Name == name {
			found = true
		}
	}
	if !found {
		t.Errorf("List did not return %s: %+v", name, infos)
	}

	res, err := exe.Run(ctx, executor.Command{Name: "echo", Args: []string{"hello"}})
	if err != nil {
		t.Fatalf("Run echo: %v", err)
	}
	if string(res.Stdout) != "hello\n" || res.ExitCode != 0 {
		t.Errorf("echo: stdout %q exit %d", res.Stdout, res.ExitCode)
	}

	res, err = exe.Run(ctx, executor.Command{Name: "sh", Args: []string{"-c", "exit 3"}})
	if err != nil {
		t.Fatalf("Run exit 3: %v", err)
	}
	if res.ExitCode != 3 {
		t.Errorf("exit code = %d, want 3", res.ExitCode)
	}

	err = exe.LookPath(ctx, "sh")
	if err != nil {
		t.Errorf("LookPath sh: %v", err)
	}
	err = exe.LookPath(ctx, "agc-no-such-command")
	if ez.ErrorCode(err) != ez.ENOTFOUND {
		t.Errorf("LookPath missing: %v, want ENOTFOUND", err)
	}

	path := exe.TempDir() + "/agc-test.txt"
	err = exe.WriteFile(ctx, path, []byte("contents\n"))
	if err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	data, err := exe.ReadFile(ctx, path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(data) != "contents\n" {
		t.Errorf("ReadFile = %q", data)
	}
	err = exe.RemoveFile(ctx, path)
	if err != nil {
		t.Errorf("RemoveFile: %v", err)
	}

	term, err := exe.StartTerminal(ctx, executor.Command{Name: "bash"})
	if err != nil {
		t.Fatalf("StartTerminal: %v", err)
	}
	terms, err := exe.ListTerminals(ctx)
	if err != nil {
		t.Fatalf("ListTerminals: %v", err)
	}
	found = false
	for _, listed := range terms {
		if listed.ID == term.ID && listed.Running {
			found = true
		}
	}
	if !found {
		t.Errorf("ListTerminals did not return running %s: %+v", term.ID, terms)
	}

	stream, err := exe.AttachTerminal(ctx, term.ID, 4096)
	if err != nil {
		t.Fatalf("AttachTerminal: %v", err)
	}
	defer stream.Close()
	err = stream.Write(ctx, []byte("echo hi\n"))
	if err != nil {
		t.Fatalf("Write: %v", err)
	}

	// The TTY echoes the typed line, so wait for "hi" on a line of its own.
	readCtx, readCancel := context.WithTimeout(ctx, 30*time.Second)
	defer readCancel()
	var out strings.Builder
	for !hasLine(out.String(), "hi") {
		chunk, err := stream.Recv(readCtx)
		if err != nil {
			t.Fatalf("Recv: %v (output so far %q)", err, out.String())
		}
		out.Write(chunk)
	}
}

func TestHasLine(t *testing.T) {
	echoed := "agent@host:~$ echo hi\r\n\x1b[?2004l\rhi\r\n\x1b[?2004h\x1b]0;agent@host: ~\aagent@host:~$ "
	if !hasLine(echoed, "hi") {
		t.Errorf("hasLine missed hi in %q", echoed)
	}
	if hasLine("agent@host:~$ echo hi\r\n", "hi") {
		t.Error("hasLine matched the echoed command")
	}
}

var ansiEscape = regexp.MustCompile("\x1b\\[[0-9;?]*[a-zA-Z]|\x1b\\][^\a]*\a")

// hasLine reports whether terminal output contains want as a whole line,
// ignoring escape sequences and carriage returns.
func hasLine(output string, want string) bool {
	plain := ansiEscape.ReplaceAllString(output, "")
	plain = strings.ReplaceAll(plain, "\r", "\n")
	for _, line := range strings.Split(plain, "\n") {
		if strings.TrimSpace(line) == want {
			return true
		}
	}
	return false
}

// TestSandboxSecret checks that a secret value never reaches the guest or the
// runtime's state on disk, and that Start passes it again on every boot.
func TestSandboxSecret(t *testing.T) {
	name, image := integrationSetup(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	value := "ref-test-value"
	secrets := []Secret{{Name: "AGC_TEST_SECRET", Value: value, Hosts: []string{"example.com"}}}

	var exe *Executor
	removeOnCleanup(t, name, &exe)
	exe, err := Create(ctx, Spec{Name: name, Image: image, MemoryMiB: 1024, Secrets: secrets})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	checkPlaceholder(ctx, t, exe, value)

	home := os.Getenv("MSB_HOME")
	if home == "" {
		userHome, err := os.UserHomeDir()
		if err != nil {
			t.Fatal(err)
		}
		home = filepath.Join(userHome, ".microsandbox")
	}
	filepath.WalkDir(home, func(path string, d fs.DirEntry, err error) error {
		if err != nil || !d.Type().IsRegular() {
			return nil
		}
		info, err := d.Info()
		if err != nil || info.Size() > 50<<20 {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		if bytes.Contains(data, []byte(value)) {
			t.Errorf("secret value found in %s", path)
		}
		return nil
	})

	err = exe.Detach(ctx)
	if err != nil {
		t.Fatalf("Detach: %v", err)
	}
	exe = nil
	err = Stop(ctx, name)
	if err != nil {
		t.Fatalf("Stop: %v", err)
	}
	err = Start(ctx, name, secrets)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	exe, err = Connect(ctx, name)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	checkPlaceholder(ctx, t, exe, value)
}

func checkPlaceholder(ctx context.Context, t *testing.T, exe *Executor, value string) {
	t.Helper()
	res, err := exe.Run(ctx, executor.Command{Name: "sh", Args: []string{"-c", `printf %s "$AGC_TEST_SECRET"`}})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	got := string(res.Stdout)
	if !strings.HasPrefix(got, "$MSB_") || strings.Contains(got, value) {
		t.Errorf("guest AGC_TEST_SECRET = %q, want a $MSB_ placeholder", got)
	}
}
