package cli

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/vanclief/agent-composer/core/resources/sandboxes"
)

// gcServer answers POST /api/sandboxes/gc and records each request body.
type gcServer struct {
	mu       sync.Mutex
	requests []sandboxes.GCRequest
}

func (f *gcServer) handler(t *testing.T) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method+" "+r.URL.Path != "POST /api/sandboxes/gc" {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"error":{"code":"not_found","message":"no route"}}`))
			return
		}

		request := sandboxes.GCRequest{}
		err := json.NewDecoder(r.Body).Decode(&request)
		if err != nil {
			t.Error(err)
		}
		f.mu.Lock()
		f.requests = append(f.requests, request)
		f.mu.Unlock()

		response := sandboxes.GCResponse{
			DryRun:    request.DryRun,
			Keep:      "48h0m0s",
			Sandboxes: []sandboxes.Collected{{Name: "fix-login", Task: "fix-login", DoneFor: "50h0m0s", Disk: "20G", DiskMiB: 20480}},
		}
		if !request.DryRun {
			response.Freed = 1
			response.FreedDisk = "20G"
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	})
}

func TestSandboxGCAgainstAServer(t *testing.T) {
	fake := &gcServer{}
	server := httptest.NewServer(fake.handler(t))
	defer server.Close()

	out, err := runAgc(t, "sandbox", "gc", "--server", server.URL, "--dry-run", "--keep", "48h")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"fix-login  fix-login  50h0m0s   20G", "Would delete 1 sandboxes, 20G of disk"} {
		if !strings.Contains(out, want) {
			t.Fatalf("dry run output %q lacks %q", out, want)
		}
	}

	t.Setenv(serverEnv, server.URL)
	out, err = runAgc(t, "sandbox", "gc")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Deleted 1 sandboxes, 20G of disk; volumes are kept.") {
		t.Fatalf("gc output %q", out)
	}

	want := []sandboxes.GCRequest{{DryRun: true, Keep: "48h"}, {}}
	if !reflect.DeepEqual(fake.requests, want) {
		t.Fatalf("requests %+v, want %+v", fake.requests, want)
	}
}
