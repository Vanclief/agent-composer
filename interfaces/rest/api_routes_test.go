package rest

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"

	workflowapi "github.com/vanclief/agent-composer/core/resources/workflow"
	"github.com/vanclief/agent-composer/interfaces/rest/handler"
	restserver "github.com/vanclief/agent-composer/interfaces/rest/server"
	workflowruntime "github.com/vanclief/agent-composer/workflow"
)

func TestListWorkflowsRoute(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("AGENT_COMPOSER_HOME", homeDir)

	workflowDir := filepath.Join(homeDir, "workflows")
	err := os.MkdirAll(workflowDir, 0755)
	if err != nil {
		t.Fatalf("mkdir workflow dir: %v", err)
	}

	spec := `
workflow:
  id: route_workflow
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

	err = os.WriteFile(filepath.Join(workflowDir, "route-workflow.yaml"), []byte(spec), 0644)
	if err != nil {
		t.Fatalf("write workflow: %v", err)
	}

	e := echo.New()
	server := &restserver.Server{
		WorkflowAPI: &workflowapi.API{},
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

	if response.Workflows[0].ID != "route_workflow" {
		t.Fatalf("unexpected workflow id: %q", response.Workflows[0].ID)
	}

	request = httptest.NewRequest(http.MethodGet, "/api/workflows/route_workflow", nil)
	recorder = httptest.NewRecorder()

	e.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", recorder.Code, recorder.Body.String())
	}

	getResponse := struct {
		WorkflowID string `json:"workflow_id"`
		Spec       string `json:"spec"`
	}{}

	err = json.Unmarshal(recorder.Body.Bytes(), &getResponse)
	if err != nil {
		t.Fatalf("decode get response: %v", err)
	}

	if getResponse.WorkflowID != "route_workflow" {
		t.Fatalf("unexpected get workflow id: %q", getResponse.WorkflowID)
	}

	if strings.TrimSpace(getResponse.Spec) != strings.TrimSpace(spec) {
		t.Fatalf("unexpected workflow spec: %q", getResponse.Spec)
	}
}
