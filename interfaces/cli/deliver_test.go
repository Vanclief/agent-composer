package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vanclief/ez"

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

const cliReport = `{"criteria":[{"met":"yes","note":"tested"}],"changes":{"branch":"agc/fix-login","sha":"abc1234"}}`

// runAgcStdin runs agc with stdin set to input and returns stdout,
// stderr and the error.
func runAgcStdin(t *testing.T, input string, args ...string) (string, string, error) {
	t.Helper()

	app := newApp()
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	app.Writer = out
	app.ErrWriter = errOut
	app.Reader = strings.NewReader(input)

	err := app.RunContext(context.Background(), append([]string{"agc"}, args...))

	return out.String(), errOut.String(), err
}

// Inside a sandbox, --report sends the file's JSON as it is over the
// socket, from a file or from stdin.
func TestTaskDeliverSendsAReportOverTheSocket(t *testing.T) {
	bodies := []map[string]json.RawMessage{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := map[string]json.RawMessage{}
		err := json.NewDecoder(r.Body).Decode(&body)
		if err != nil {
			t.Error(err)
		}
		bodies = append(bodies, body)

		httpapi.WriteJSON(w, http.StatusOK, tasks.TaskView{Name: "fix-login", Status: tasks.StatusReview})
	}))
	defer server.Close()

	t.Setenv("AGC_SERVER", "")
	t.Setenv("AGC_TASK", "fix-login")
	t.Setenv("AGC_MSG_URL", server.URL)

	file := filepath.Join(t.TempDir(), "report.json")
	err := os.WriteFile(file, []byte("\n"+cliReport+"\n"), 0o600)
	if err != nil {
		t.Fatal(err)
	}

	_, err = runAgc(t, "task", "deliver", "--report", file, "Fixed 2FA.")
	if err != nil {
		t.Fatal(err)
	}

	_, _, err = runAgcStdin(t, cliReport, "task", "deliver", "--report", "-", "Fixed 2FA again.")
	if err != nil {
		t.Fatal(err)
	}

	if len(bodies) != 2 {
		t.Fatalf("bodies = %v", bodies)
	}
	for _, body := range bodies {
		if string(body["report"]) != cliReport {
			t.Fatalf("report = %s, want %s", body["report"], cliReport)
		}
	}

	_, _, err = runAgcStdin(t, "{not json", "task", "deliver", "--report", "-", "x")
	if ez.ErrorCode(err) != ez.EINVALID || !strings.Contains(ez.ErrorMessage(err), "not valid JSON") {
		t.Fatalf("err %v", err)
	}

	_, err = runAgc(t, "task", "deliver", "--report", filepath.Join(t.TempDir(), "missing.json"), "x")
	if ez.ErrorCode(err) != ez.EINVALID || !strings.Contains(ez.ErrorMessage(err), "cannot read the report") {
		t.Fatalf("err %v", err)
	}
	if len(bodies) != 2 {
		t.Fatalf("a bad report must not be sent: %v", bodies)
	}
}

// Against a server, --report goes in the deliver body, and the server's
// refusal is returned as it is.
func TestTaskDeliverSendsAReportToTheServer(t *testing.T) {
	var got tasks.DeliverRequest
	refusal := "report has 1 criteria but the task has 2 accept criteria: give one per \"Accepted when\" line, in order"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/tasks/fix-login/deliver" {
			t.Errorf("request %s %s", r.Method, r.URL.Path)
		}

		err := json.NewDecoder(r.Body).Decode(&got)
		if err != nil {
			t.Error(err)
		}

		if got.Summary == "refuse" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]string{"message": refusal}})
			return
		}

		httpapi.WriteJSON(w, http.StatusOK, tasks.TaskView{Name: "fix-login", Status: tasks.StatusReview, Report: got.Report})
	}))
	defer server.Close()

	t.Setenv("AGC_TASK", "")
	t.Setenv("AGC_MSG_URL", "")

	_, _, err := runAgcStdin(t, cliReport, "task", "deliver", "--server", server.URL, "--task", "fix-login", "--report", "-", "Fixed 2FA.")
	if err != nil {
		t.Fatal(err)
	}
	if got.Report == nil || got.Report.Criteria[0].Met != "yes" || got.Report.Changes.SHA != "abc1234" {
		t.Fatalf("request = %+v", got)
	}

	_, stderr, err := runAgcStdin(t, cliReport, "task", "deliver", "--server", server.URL, "--task", "fix-login", "--report", "-", "refuse")
	if err == nil || stderr != refusal+"\n" {
		t.Fatalf("err %v, stderr %q, want the server's message %q", err, stderr, refusal)
	}
}

// Inside a sandbox, the refusal from the socket is the error's message,
// which agc prints as it is.
func TestTaskDeliverOverTheSocketReturnsTheRefusal(t *testing.T) {
	refusal := `report criteria[0].met is "done": use yes, no or partly`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		httpapi.WriteError(w, ez.New(ez.EINVALID, refusal, nil))
	}))
	defer server.Close()

	t.Setenv("AGC_SERVER", "")
	t.Setenv("AGC_TASK", "fix-login")
	t.Setenv("AGC_MSG_URL", server.URL)

	_, _, err := runAgcStdin(t, `{"criteria":[{"met":"done"}]}`, "task", "deliver", "--report", "-", "x")
	if ez.ErrorCode(err) != ez.EINVALID || ez.ErrorMessage(err) != refusal {
		t.Fatalf("err %v, message %q", err, ez.ErrorMessage(err))
	}
}
