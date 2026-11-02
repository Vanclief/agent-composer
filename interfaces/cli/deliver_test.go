package cli

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/vanclief/agent-composer/core/resources/tasks"
	"github.com/vanclief/agent-composer/messaging/httpapi"
)

// Inside a sandbox, agc task deliver goes over the messaging socket
// (AGC_MSG_URL stands in for vsock here), for the sandbox's own task only.
func TestTaskDeliverInsideASandboxUsesTheSocket(t *testing.T) {
	var got httpapi.DeliverRequest
	paths := []string{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.Method+" "+r.URL.Path)

		err := json.NewDecoder(r.Body).Decode(&got)
		if err != nil {
			t.Error(err)
		}

		httpapi.WriteJSON(w, http.StatusOK, tasks.TaskView{Name: "fix-login", Status: tasks.StatusReview})
	}))
	defer server.Close()

	t.Setenv("AGC_SERVER", "")
	t.Setenv("AGC_TASK", "fix-login")
	t.Setenv("AGC_MSG_URL", server.URL)

	out, err := runAgc(t, "task", "deliver", "Fixed 2FA,", "branch agc/fix-login")
	if err != nil {
		t.Fatal(err)
	}

	if len(paths) != 1 || paths[0] != "POST /task/deliver" || got.Summary != "Fixed 2FA, branch agc/fix-login" {
		t.Fatalf("paths %q, request %+v", paths, got)
	}
	if !strings.HasPrefix(out, "fix-login  review") {
		t.Fatalf("output %q", out)
	}

	_, err = runAgc(t, "task", "deliver", "--task", "other", "x")
	if err == nil || !strings.Contains(err.Error(), "only its own task") {
		t.Fatalf("err %v", err)
	}
	if len(paths) != 1 {
		t.Fatalf("paths %q", paths)
	}
}
