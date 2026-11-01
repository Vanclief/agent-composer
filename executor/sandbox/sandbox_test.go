//go:build cgo

package sandbox

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/vanclief/agent-composer/executor"
	"github.com/vanclief/ez"
)

// TestSandbox runs against a real microsandbox runtime. It needs a host with
// KVM (or Apple Silicon) and the agent image loaded with msb.
func TestSandbox(t *testing.T) {
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
	name := "agc-test-" + hex.EncodeToString(suffix)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	var exe *Executor
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), time.Minute)
		defer cleanupCancel()
		if exe != nil {
			exe.Detach(cleanupCtx)
		}
		err := Stop(cleanupCtx, name)
		if err != nil {
			t.Logf("Stop: %v", err)
		}
		err = Remove(cleanupCtx, name)
		if err != nil {
			t.Errorf("Remove: %v", err)
		}
	})

	exe, err = Create(ctx, Spec{Name: name, Image: image, MemoryMiB: 1024, Labels: map[string]string{"agc.test": name}})
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
	for !strings.Contains(out.String(), "\nhi\r") {
		chunk, err := stream.Recv(readCtx)
		if err != nil {
			t.Fatalf("Recv: %v (output so far %q)", err, out.String())
		}
		out.Write(chunk)
	}
}
