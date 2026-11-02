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

// The order TASK.md, the guest readme and ADR 0010 give: the summary
// first, --report <file> after it. urfave/cli stops reading flags at the
// summary, so leafArgs must pick --report out; the summary carries no flag
// text and the report is sent.
func TestTaskDeliverReadsAReportAfterTheSummary(t *testing.T) {
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
	err := os.WriteFile(file, []byte(cliReport), 0o600)
	if err != nil {
		t.Fatal(err)
	}

	_, err = runAgc(t, "task", "deliver", "Fixed 2FA; all checks pass.", "--report", file)
	if err != nil {
		t.Fatal(err)
	}

	_, err = runAgc(t, "task", "deliver", "Fixed 2FA again.", "--report="+file)
	if err != nil {
		t.Fatal(err)
	}

	_, err = runAgc(t, "task", "deliver", "--report", file, "Fixed 2FA, flag first.")
	if err != nil {
		t.Fatal(err)
	}

	want := []string{`"Fixed 2FA; all checks pass."`, `"Fixed 2FA again."`, `"Fixed 2FA, flag first."`}
	if len(bodies) != len(want) {
		t.Fatalf("bodies = %v", bodies)
	}
	for i, body := range bodies {
		if string(body["summary"]) != want[i] {
			t.Fatalf("summary = %s, want %s", body["summary"], want[i])
		}
		if string(body["report"]) != cliReport {
			t.Fatalf("report = %s, want %s", body["report"], cliReport)
		}
	}

	_, err = runAgc(t, "task", "deliver", "Fixed 2FA.", "--report")
	if ez.ErrorCode(err) != ez.EINVALID || !strings.Contains(ez.ErrorMessage(err), "--report needs a value") {
		t.Fatalf("err %v", err)
	}
}

// Inside a sandbox, --report with shots reads each shot's PNG from its
// file and sends it with the delivery, in files under its label; a shot
// whose file cannot be read stops agc before anything is sent.
func TestTaskDeliverSendsShotsOverTheSocket(t *testing.T) {
	var got httpapi.DeliverRequest
	sent := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sent++

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

	dir := t.TempDir()
	before := []byte("\x89PNG\r\n\x1a\nbefore")
	after := []byte("\x89PNG\r\n\x1a\nafter")
	for name, data := range map[string][]byte{"before.png": before, "after.png": after} {
		err := os.WriteFile(filepath.Join(dir, name), data, 0o600)
		if err != nil {
			t.Fatal(err)
		}
	}

	report := `{"criteria":[{"met":"yes"}],"shots":[` +
		`{"label":"before","file":"` + filepath.Join(dir, "before.png") + `"},` +
		`{"label":"after","file":"` + filepath.Join(dir, "after.png") + `"}]}`

	_, _, err := runAgcStdin(t, report, "task", "deliver", "--report", "-", "Moved the button.")
	if err != nil {
		t.Fatal(err)
	}

	if sent != 1 || string(got.Report) != report || len(got.Files) != 2 {
		t.Fatalf("sent %d, request %+v", sent, got)
	}
	if !bytes.Equal(got.Files["before"], before) || !bytes.Equal(got.Files["after"], after) {
		t.Fatalf("files %q", got.Files)
	}

	missing := `{"criteria":[{"met":"yes"}],"shots":[{"label":"after","file":"` + filepath.Join(dir, "nope.png") + `"}]}`
	_, _, err = runAgcStdin(t, missing, "task", "deliver", "--report", "-", "x")
	if ez.ErrorCode(err) != ez.EINVALID || !strings.Contains(ez.ErrorMessage(err), "cannot read the shot after") {
		t.Fatalf("err %v", err)
	}
	if sent != 1 {
		t.Fatalf("a shot that cannot be read must not be sent: sent %d", sent)
	}
}
