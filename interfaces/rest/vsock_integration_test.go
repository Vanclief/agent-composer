package rest

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"math/rand/v2"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/vanclief/agent-composer/core/resources/sandboxes"
	"github.com/vanclief/agent-composer/core/resources/tasks"
	"github.com/vanclief/agent-composer/executor"
	"github.com/vanclief/agent-composer/executor/sandbox"
)

// noisePNG encodes a square PNG of random pixels, which PNG cannot
// compress, side pixels wide.
func noisePNG(t *testing.T, side int) []byte {
	t.Helper()

	random := rand.New(rand.NewPCG(1, uint64(side)))
	img := image.NewNRGBA(image.Rect(0, 0, side, side))
	for x := 0; x < side; x++ {
		for y := 0; y < side; y++ {
			img.Set(x, y, color.NRGBA{R: uint8(random.Uint32()), G: uint8(random.Uint32()), B: uint8(random.Uint32()), A: 255})
		}
	}

	out := &bytes.Buffer{}
	err := png.Encode(out, img)
	if err != nil {
		t.Fatal(err)
	}

	return out.Bytes()
}

// buildGuestAgc builds this tree's agc for the guest, so the test runs
// the client under test, not the image's.
func buildGuestAgc(t *testing.T) []byte {
	t.Helper()

	out := filepath.Join(t.TempDir(), "agc")
	cmd := exec.Command("go", "build", "-ldflags", "-s -w", "-o", out, ".")
	cmd.Dir = filepath.Join("..", "..")
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0", "GOOS=linux", "GOARCH="+runtime.GOARCH)

	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go build: %v\n%s", err, output)
	}

	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}

	return data
}

// A delivery with a 1.5 MB and a 100 KB shot, from inside a real
// sandbox, over its vsock socket: the vsock forwarder lets a body of at
// most 256 KiB through, so agc sends the shots in pieces first (ADR 0010,
// "Delivery report"). Runs on a KVM host only:
//
//	AGC_SANDBOX_TEST=1 go test ./interfaces/rest/ -run TestDeliverShotsFromASandbox -v
//
// AGC_SANDBOX_IMAGE picks the image (default: the spec's default).
func TestDeliverShotsFromASandbox(t *testing.T) {
	if os.Getenv("AGC_SANDBOX_TEST") != "1" {
		t.Skip("set AGC_SANDBOX_TEST=1 to run against a microsandbox runtime")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	agc := buildGuestAgc(t)

	before := noisePNG(t, 707)
	after := noisePNG(t, 182)
	if len(before) < 1400*1000 || len(before) > 1600*1000 || len(after) < 90*1000 || len(after) > 110*1000 {
		t.Fatalf("shots are %d and %d bytes, want about 1.5 MB and 100 KB", len(before), len(after))
	}

	routes := newTaskRoutes(t, sandboxes.SandboxRuntime{}, "")

	sandboxSpec := map[string]any{"command": "sleep 3600"}
	image := os.Getenv("AGC_SANDBOX_IMAGE")
	if image != "" {
		sandboxSpec["image"] = image
	}

	name := "vsock-shots-" + time.Now().Format("150405")
	create, err := json.Marshal(map[string]any{"name": name, "title": "Vsock shots", "brief": "Deliver two shots.", "manager": "orchestrator", "sandbox": sandboxSpec})
	if err != nil {
		t.Fatal(err)
	}

	status, body := routes.do(http.MethodPost, "/api/tasks", string(create))
	if status != http.StatusOK {
		t.Fatalf("create: status %d, body %s", status, body)
	}

	t.Cleanup(func() {
		cleanup, cleanupCancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cleanupCancel()

		_, err := routes.server.SandboxesAPI.Delete(cleanup, nil, &sandboxes.DeleteRequest{Name: name})
		if err != nil {
			t.Logf("delete sandbox %s: %v (its volume agc-%s is kept)", name, err, name)
		}
	})

	routes.server.SandboxesAPI.Wait()

	view, err := routes.server.TasksAPI.Get(ctx, nil, &tasks.GetRequest{Name: name})
	if err != nil {
		t.Fatal(err)
	}
	if len(view.Sandboxes) != 1 || view.Sandboxes[0].Status != "running" {
		t.Fatalf("sandbox did not boot: %+v", view.Sandboxes)
	}

	exe, err := sandbox.Connect(ctx, name)
	if err != nil {
		t.Fatal(err)
	}
	defer exe.Detach(ctx)

	files := map[string][]byte{
		"/tmp/agc-under-test": agc,
		"/tmp/before.png":     before,
		"/tmp/after.png":      after,
		"/tmp/report.json":    []byte(`{"shots":[{"label":"before","file":"/tmp/before.png"},{"label":"after","file":"/tmp/after.png"}]}`),
	}
	for path, data := range files {
		err = exe.WriteFile(ctx, path, data)
		if err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}

	res, err := exe.Run(ctx, executor.Command{Name: "chmod", Args: []string{"+x", "/tmp/agc-under-test"}})
	if err != nil || res.ExitCode != 0 {
		t.Fatalf("chmod: %v %+v", err, res)
	}

	start := time.Now()
	res, err = exe.Run(ctx, executor.Command{
		Name: "/tmp/agc-under-test",
		Args: []string{"task", "deliver", "--report", "/tmp/report.json", "Two shots over vsock."},
		Env:  map[string]string{"AGC_TASK": name},
	})
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("agc task deliver: %v", err)
	}
	t.Logf("agc task deliver with %d + %d bytes of shots took %s", len(before), len(after), elapsed)
	if res.ExitCode != 0 {
		t.Fatalf("agc task deliver exited %d after %s\nstdout: %s\nstderr: %s", res.ExitCode, elapsed, res.Stdout, res.Stderr)
	}
	if elapsed > 30*time.Second {
		t.Fatalf("agc task deliver took %s, want under 30s", elapsed)
	}

	shots := map[string][]byte{"before": before, "after": after}
	for label, want := range shots {
		status, served := routes.do(http.MethodGet, "/api/tasks/"+name+"/shots/"+label+".png", "")
		if status != http.StatusOK {
			t.Fatalf("GET shot %s: status %d, body %s", label, status, served)
		}
		if served != string(want) {
			t.Fatalf("shot %s: served %d bytes, delivered %d", label, len(served), len(want))
		}
	}

	view, err = routes.server.TasksAPI.Get(ctx, nil, &tasks.GetRequest{Name: name})
	if err != nil {
		t.Fatal(err)
	}
	if view.Status != "review" || !strings.Contains(view.Summary, "Two shots") {
		t.Fatalf("task after delivery: status %s, summary %q", view.Status, view.Summary)
	}
}
