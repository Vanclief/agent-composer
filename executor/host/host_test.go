package host

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/vanclief/agent-composer/executor"
	"github.com/vanclief/ez"
)

func TestRunCapturesStdout(t *testing.T) {
	result, err := New().Run(context.Background(), executor.Command{Name: "echo", Args: []string{"hello", "world"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if string(result.Stdout) != "hello world\n" {
		t.Fatalf("unexpected stdout: %q", result.Stdout)
	}

	if result.ExitCode != 0 {
		t.Fatalf("unexpected exit code: %d", result.ExitCode)
	}
}

func TestRunReportsNonZeroExitWithoutError(t *testing.T) {
	result, err := New().Run(context.Background(), executor.Command{Name: "sh", Args: []string{"-c", "echo oops >&2 && exit 3"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.ExitCode != 3 {
		t.Fatalf("unexpected exit code: %d", result.ExitCode)
	}

	if string(result.Stderr) != "oops\n" {
		t.Fatalf("unexpected stderr: %q", result.Stderr)
	}
}

func TestRunAddsEnv(t *testing.T) {
	cmd := executor.Command{
		Name: "sh",
		Args: []string{"-c", `printf %s "$AGC_HOST_TEST"`},
		Env:  map[string]string{"AGC_HOST_TEST": "from-env"},
	}

	result, err := New().Run(context.Background(), cmd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if string(result.Stdout) != "from-env" {
		t.Fatalf("unexpected stdout: %q", result.Stdout)
	}
}

func TestRunWritesStdin(t *testing.T) {
	result, err := New().Run(context.Background(), executor.Command{Name: "cat", Stdin: []byte("piped")})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if string(result.Stdout) != "piped" {
		t.Fatalf("unexpected stdout: %q", result.Stdout)
	}
}

func TestRunUsesDir(t *testing.T) {
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	result, err := New().Run(context.Background(), executor.Command{Name: "pwd", Args: []string{"-P"}, Dir: dir})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if string(result.Stdout) != dir+"\n" {
		t.Fatalf("unexpected stdout: %q", result.Stdout)
	}
}

func TestRunMissingBinaryIsInternalError(t *testing.T) {
	_, err := New().Run(context.Background(), executor.Command{Name: "agc-missing-binary-for-test"})
	if ez.ErrorCode(err) != ez.EINTERNAL {
		t.Fatalf("expected EINTERNAL, got %v", err)
	}
}

func TestRunCanceledContextIsUnavailable(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := New().Run(ctx, executor.Command{Name: "sh", Args: []string{"-c", "sleep 5"}})
	if ez.ErrorCode(err) != ez.EUNAVAILABLE {
		t.Fatalf("expected EUNAVAILABLE, got %v", err)
	}
}

func TestLookPath(t *testing.T) {
	err := New().LookPath(context.Background(), "sh")
	if err != nil {
		t.Fatalf("expected sh on PATH: %v", err)
	}

	err = New().LookPath(context.Background(), "agc-missing-binary-for-test")
	if ez.ErrorCode(err) != ez.ENOTFOUND {
		t.Fatalf("expected ENOTFOUND, got %v", err)
	}
}

func TestFileRoundTrip(t *testing.T) {
	ctx := context.Background()
	exec := New()
	path := filepath.Join(t.TempDir(), "file.txt")

	err := exec.WriteFile(ctx, path, []byte("content"))
	if err != nil {
		t.Fatalf("unexpected write error: %v", err)
	}

	data, err := exec.ReadFile(ctx, path)
	if err != nil {
		t.Fatalf("unexpected read error: %v", err)
	}

	if string(data) != "content" {
		t.Fatalf("unexpected content: %q", data)
	}

	err = exec.RemoveFile(ctx, path)
	if err != nil {
		t.Fatalf("unexpected remove error: %v", err)
	}

	_, err = exec.ReadFile(ctx, path)
	if err == nil {
		t.Fatal("expected read error after remove")
	}
}
