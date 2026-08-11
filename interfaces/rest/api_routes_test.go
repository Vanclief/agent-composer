package rest

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/sqlitedialect"

	workflowapi "github.com/vanclief/agent-composer/core/resources/workflow"
	"github.com/vanclief/agent-composer/interfaces/rest/handler"
	restserver "github.com/vanclief/agent-composer/interfaces/rest/server"
	workflowmodels "github.com/vanclief/agent-composer/models/workflow"
	workflowruntime "github.com/vanclief/agent-composer/workflow"
	_ "modernc.org/sqlite"
)

func TestListWorkflowsRoute(t *testing.T) {
	sqldb, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	// Every pooled connection would get its own :memory: database, so
	// the pool must stay at one connection.
	sqldb.SetMaxOpenConns(1)
	t.Cleanup(func() {
		_ = sqldb.Close()
	})

	db := bun.NewDB(sqldb, sqlitedialect.New())
	ctx := context.Background()

	tables := []interface{}{
		(*workflowmodels.Workflow)(nil),
		(*workflowmodels.WorkflowVersion)(nil),
	}
	for _, table := range tables {
		_, err = db.NewCreateTable().Model(table).Exec(ctx)
		if err != nil {
			t.Fatal(err)
		}
	}

	spec := `
workflow:
  slug: route_workflow
  version: "1"
  description: Route workflow.
  inputs:
    prompt: string
  outputs:
    answer:
      schema: string
      from: answer.out
nodes:
  answer:
    kind: inference
    outputs:
      out: string
    config:
      instruction: Answer.
flow:
  instances:
    answer:
      node: answer
      inputs:
        prompt: workflow_input.prompt
`

	record := &workflowmodels.Workflow{
		Slug:    "route_workflow",
		Version: 1,
		Spec:    spec,
	}
	err = record.Insert(ctx, db)
	if err != nil {
		t.Fatal(err)
	}

	e := echo.New()
	server := &restserver.Server{
		WorkflowAPI: &workflowapi.API{
			Registry: workflowruntime.NewRegistry(db),
		},
	}
	addAPIRoutes(e, handler.NewHandler(server))

	request := httptest.NewRequest(http.MethodGet, "/api/workflows", nil)
	recorder := httptest.NewRecorder()

	e.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", recorder.Code, recorder.Body.String())
	}

	response := struct {
		Workflows []workflowruntime.WorkflowSummary `json:"workflows"`
	}{}

	err = json.Unmarshal(recorder.Body.Bytes(), &response)
	if err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if len(response.Workflows) != 1 {
		t.Fatalf("expected 1 workflow, got %d", len(response.Workflows))
	}

	if response.Workflows[0].Slug != "route_workflow" {
		t.Fatalf("unexpected workflow id: %q", response.Workflows[0].Slug)
	}

	request = httptest.NewRequest(http.MethodGet, "/api/workflows/route_workflow", nil)
	recorder = httptest.NewRecorder()

	e.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", recorder.Code, recorder.Body.String())
	}

	getResponse := struct {
		WorkflowSlug string `json:"workflow_slug"`
		Spec         string `json:"spec"`
	}{}

	err = json.Unmarshal(recorder.Body.Bytes(), &getResponse)
	if err != nil {
		t.Fatalf("decode get response: %v", err)
	}

	if getResponse.WorkflowSlug != "route_workflow" {
		t.Fatalf("unexpected get workflow id: %q", getResponse.WorkflowSlug)
	}

	if strings.TrimSpace(getResponse.Spec) != strings.TrimSpace(spec) {
		t.Fatalf("unexpected workflow spec: %q", getResponse.Spec)
	}
}
