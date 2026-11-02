package tasks

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/vanclief/ez"

	"github.com/vanclief/agent-composer/messaging"
	taskmodels "github.com/vanclief/agent-composer/models/task"
)

func sampleReport() *taskmodels.Report {
	files := 41
	return &taskmodels.Report{
		Criteria: []taskmodels.Criterion{
			{Met: "yes", Note: " GET /api/agents returns the list "},
			{Met: "partly", Note: "the panel half is panel-review"},
			{Met: "no", Note: ""},
		},
		Changes: &taskmodels.Changes{Branch: "agc/agents-core", SHA: "efbec13", Files: &files},
		Checks:  []taskmodels.Check{{Name: "go test ./...", Passed: true}, {Name: "make panel-check", Passed: false, Note: "no browser"}},
		LeftOut: []taskmodels.LeftOut{{What: "DM channel for agents", Why: "needs ADR 0011"}},
		Needs:   []string{"Decide whether never-kept agents are swept"},
	}
}

func lastMessage(t *testing.T, env *testEnv, channel string) string {
	t.Helper()

	messages, err := env.msg.Read(context.Background(), messaging.Caller{Human: true}, channel, 0, 50)
	if err != nil {
		t.Fatal(err)
	}

	return messages[len(messages)-1].Body
}

func TestDeliverWithAReportStoresRendersAndKeepsIt(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()

	accept := []string{"GET /api/agents lists agents", "the panel shows them", "agents sleep when idle"}
	_, err := env.api.Create(ctx, nil, &CreateRequest{Manager: "orchestrator", Brief: "b", Name: "agents-core", Accept: accept})
	if err != nil {
		t.Fatal(err)
	}

	view, err := env.api.Deliver(ctx, nil, &DeliverRequest{Name: "agents-core", Summary: "Agents core is in.", Report: sampleReport()})
	if err != nil {
		t.Fatal(err)
	}
	if view.Report == nil || len(view.Report.Criteria) != 3 || view.Report.Criteria[0].Note != "GET /api/agents returns the list" || view.Report.Changes.SHA != "efbec13" {
		t.Fatalf("delivered report = %+v", view.Report)
	}

	got, err := env.api.Get(ctx, nil, &GetRequest{Name: "agents-core"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Report == nil || got.Report.Needs[0] != "Decide whether never-kept agents are swept" || *got.Report.Changes.Files != 41 {
		t.Fatalf("stored report = %+v", got.Report)
	}

	listed, err := env.api.List(ctx, nil, &ListRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if listed.Tasks[0].Report == nil || len(listed.Tasks[0].Report.Checks) != 2 {
		t.Fatalf("listed report = %+v", listed.Tasks[0].Report)
	}

	want := "Delivered: Agents core is in.\n" +
		"\n" +
		"- [x] GET /api/agents lists agents — GET /api/agents returns the list\n" +
		"- [~] the panel shows them — the panel half is panel-review\n" +
		"- [ ] agents sleep when idle\n" +
		"\n" +
		"Changes: agc/agents-core @ efbec13 (41 files)\n" +
		"\n" +
		"Checks:\n" +
		"- ✓ go test ./...\n" +
		"- ✗ make panel-check — no browser\n" +
		"\n" +
		"Left out:\n" +
		"- DM channel for agents: needs ADR 0011\n" +
		"\n" +
		"Needs you:\n" +
		"- Decide whether never-kept agents are swept"
	body := lastMessage(t, env, "#agents-core")
	if body != want {
		t.Fatalf("message:\n%s\n--- want\n%s", body, want)
	}

	view, err = env.api.Reject(ctx, nil, &RejectRequest{Name: "agents-core", By: "orchestrator", Why: "the panel half is missing"})
	if err != nil {
		t.Fatal(err)
	}
	if view.DeliveredAt != nil || view.Report == nil || len(view.Report.Criteria) != 3 {
		t.Fatalf("a rejection keeps the report: %+v", view.Report)
	}

	// A delivery replaces the report with its own, none here.
	view, err = env.api.Deliver(ctx, nil, &DeliverRequest{Name: "agents-core", Summary: "Again."})
	if err != nil {
		t.Fatal(err)
	}
	if view.Report != nil || lastMessage(t, env, "#agents-core") != "Delivered: Again." {
		t.Fatalf("report = %+v", view.Report)
	}

	data, err := json.Marshal(view)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"report":null`) {
		t.Fatalf("view JSON = %s", data)
	}
}

func TestDeliverRefusesAReportThatDoesNotMatchTheCriteria(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()

	_, err := env.api.Create(ctx, nil, &CreateRequest{Manager: "orchestrator", Brief: "b", Name: "two-criteria", Accept: []string{"a", "b"}})
	if err != nil {
		t.Fatal(err)
	}

	_, err = env.api.Deliver(ctx, nil, &DeliverRequest{Name: "two-criteria", Summary: "s", Report: sampleReport()})
	if ez.ErrorCode(err) != ez.EINVALID || !strings.Contains(ez.ErrorMessage(err), "report has 3 criteria but the task has 2 accept criteria") {
		t.Fatalf("got %v", err)
	}

	bad := sampleReport()
	bad.Criteria = bad.Criteria[:2]
	bad.Changes.SHA = "nothex!"
	_, err = env.api.Deliver(ctx, nil, &DeliverRequest{Name: "two-criteria", Summary: "s", Report: bad})
	if ez.ErrorCode(err) != ez.EINVALID || !strings.Contains(ez.ErrorMessage(err), "changes.sha") {
		t.Fatalf("got %v", err)
	}

	got, err := env.api.Get(ctx, nil, &GetRequest{Name: "two-criteria"})
	if err != nil {
		t.Fatal(err)
	}
	if got.DeliveredAt != nil || got.Report != nil {
		t.Fatalf("a refused delivery stores nothing: %+v", got)
	}

	// No criteria on the task: the report has none either.
	_, err = env.api.Create(ctx, nil, &CreateRequest{Manager: "orchestrator", Brief: "b", Name: "no-criteria"})
	if err != nil {
		t.Fatal(err)
	}

	_, err = env.api.Deliver(ctx, nil, &DeliverRequest{Name: "no-criteria", Summary: "s", Report: sampleReport()})
	if ez.ErrorCode(err) != ez.EINVALID {
		t.Fatalf("got %v", err)
	}

	view, err := env.api.Deliver(ctx, nil, &DeliverRequest{Name: "no-criteria", Summary: "Only the needs.", Report: &taskmodels.Report{Needs: []string{"Pick a name"}}})
	if err != nil {
		t.Fatal(err)
	}
	if view.Report.Criteria == nil || len(view.Report.Criteria) != 0 || view.Report.Changes != nil {
		t.Fatalf("report = %+v", view.Report)
	}
	if lastMessage(t, env, "#no-criteria") != "Delivered: Only the needs.\n\nNeeds you:\n- Pick a name" {
		t.Fatalf("message = %q", lastMessage(t, env, "#no-criteria"))
	}
}

func TestDeliverFromSandboxDecodesTheReport(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()

	_, err := env.api.Create(ctx, nil, &CreateRequest{Manager: "orchestrator", Brief: "b", Name: "socket", Accept: []string{"it works"}})
	if err != nil {
		t.Fatal(err)
	}

	_, err = env.api.deliverFromSandbox(ctx, "socket", "s", json.RawMessage(`{"criteria":[{"met":"maybe"}]}`), nil)
	if ez.ErrorCode(err) != ez.EINVALID || !strings.Contains(ez.ErrorMessage(err), `criteria[0].met is "maybe"`) {
		t.Fatalf("got %v", err)
	}

	_, err = env.api.deliverFromSandbox(ctx, "socket", "s", json.RawMessage(`{"criteria":"yes"}`), nil)
	if ez.ErrorCode(err) != ez.EINVALID {
		t.Fatalf("got %v", err)
	}

	out, err := env.api.deliverFromSandbox(ctx, "socket", "s", json.RawMessage(`{"criteria":[{"met":"yes","note":"ran it"}]}`), nil)
	if err != nil {
		t.Fatal(err)
	}
	view := out.(*TaskView)
	if view.Report == nil || view.Report.Criteria[0].Note != "ran it" {
		t.Fatalf("report = %+v", view.Report)
	}
}

func TestDeliveredMessageIsCutToFitABody(t *testing.T) {
	report := &taskmodels.Report{}
	for range taskmodels.MaxReportEntries {
		report.LeftOut = append(report.LeftOut, taskmodels.LeftOut{What: strings.Repeat("w", 1000), Why: strings.Repeat("y", 1000)})
	}

	body := DeliveredMessage(&taskmodels.Task{Summary: "Big.", Report: report})
	if len(body) > 64*1024 || !strings.HasSuffix(body, reportCut) || !strings.HasPrefix(body, "Delivered: Big.\n\nLeft out:\n- ") {
		t.Fatalf("body is %d bytes, ends %q", len(body), body[len(body)-60:])
	}
}

func TestDeliverRequestValidatesTheReportShape(t *testing.T) {
	err := DeliverRequest{Name: "x", Summary: "s", Report: &taskmodels.Report{Criteria: []taskmodels.Criterion{{Met: "nope"}}}}.Validate()
	if ez.ErrorCode(err) != ez.EINVALID || !strings.Contains(ez.ErrorMessage(err), "met") {
		t.Fatalf("got %v", err)
	}

	err = DeliverRequest{Name: "x", Summary: "s", Report: &taskmodels.Report{Checks: []taskmodels.Check{{Name: "t", Note: strings.Repeat("n", 501)}}}}.Validate()
	if ez.ErrorCode(err) != ez.EINVALID || !strings.Contains(ez.ErrorMessage(err), "at most 500") {
		t.Fatalf("got %v", err)
	}
}
