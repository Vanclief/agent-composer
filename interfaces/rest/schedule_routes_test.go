package rest

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vanclief/agent-composer/core/resources/sandboxes"
)

// ADR 0012: schedules, their run and their items.
func TestScheduleRoutes(t *testing.T) {
	routes := newTaskRoutes(t, sandboxes.SandboxRuntime{}, "")

	poll := filepath.Join(t.TempDir(), "poll.sh")
	script := "#!/bin/sh\necho '{\"key\":\"pr-1@aaa\",\"url\":\"https://github.com/o/r/pull/1\",\"title\":\"Fix the icon\"}'\n"
	err := os.WriteFile(poll, []byte(script), 0o755)
	if err != nil {
		t.Fatal(err)
	}

	encoded, err := json.Marshal(poll)
	if err != nil {
		t.Fatal(err)
	}
	template := `"task":{"title":"Review {{.title}}","brief":"Review the pull request {{.url}}.","manager":"orchestrator"}`
	body := `{"name":"pr-reviews","every":"2m",` + template + `,"poll":{"command":` + string(encoded) + `}}`

	routes.check(t, []routeCase{
		{http.MethodGet, "/api/schedules", "", http.StatusOK, `{"schedules":[]}`},
		{http.MethodGet, "/api/schedules/pr-reviews", "", http.StatusNotFound, "schedule pr-reviews not found"},
		{http.MethodPost, "/api/schedules/pr-reviews/run", "", http.StatusNotFound, "schedule pr-reviews not found"},
		{http.MethodPost, "/api/schedules", `{"name":"pr-reviews","every":"30s",` + template + `}`, http.StatusBadRequest, "shorter than the minimum"},
		{http.MethodPost, "/api/schedules", `{"name":"pr-reviews","every":"2m","task":{"title":"Review","brief":"b"}}`, http.StatusBadRequest, "manager is required"},
		{http.MethodPost, "/api/schedules", body, http.StatusOK, `"enabled":true`},
		{http.MethodPost, "/api/schedules", body, http.StatusConflict, "schedule pr-reviews already exists"},
		{http.MethodGet, "/api/schedules", "", http.StatusOK, `"name":"pr-reviews"`},
		{http.MethodGet, "/api/schedules/pr-reviews", "", http.StatusOK, `"last_run_at":null`},
		{http.MethodPost, "/api/schedules/pr-reviews/run", "", http.StatusOK, `"title":"Review Fix the icon"`},
		{http.MethodPost, "/api/schedules/pr-reviews/run", "", http.StatusOK, `"created":[],"error":""`},
		{http.MethodGet, "/api/schedules/pr-reviews/items", "", http.StatusOK, `"key":"pr-1@aaa"`},
		{http.MethodGet, "/api/tasks/review-fix-the-icon", "", http.StatusNotFound, ""},
		{http.MethodPut, "/api/schedules/pr-reviews", `{"every":"5m","enabled":false,` + template + `,"poll":{"command":` + string(encoded) + `}}`, http.StatusOK, `"next_run_at":null`},
		{http.MethodGet, "/api/schedules/pr-reviews", "", http.StatusOK, `"every":"5m","enabled":false`},
		// PUT replaces: without the poll, {{.title}} has nothing to read.
		{http.MethodPut, "/api/schedules/pr-reviews", `{"every":"5m",` + template + `}`, http.StatusBadRequest, `no entry for key \"title\"`},
		{http.MethodPut, "/api/schedules/pr-reviews", `{"every":"5m","task":{"title":"{{.title"}}`, http.StatusBadRequest, "not a valid template"},
		{http.MethodPut, "/api/schedules/nope", `{"every":"5m",` + template + `}`, http.StatusNotFound, "schedule nope not found"},
	})

	status, got := routes.do(http.MethodGet, "/api/schedules/pr-reviews", "")
	if status != http.StatusOK {
		t.Fatalf("get: %d %s", status, got)
	}
	view := struct {
		ID string `json:"id"`
	}{}
	err = json.Unmarshal([]byte(got), &view)
	if err != nil {
		t.Fatal(err)
	}

	status, got = routes.do(http.MethodGet, "/api/schedules/"+view.ID+"/items", "")
	if status != http.StatusOK || !strings.Contains(got, `"task":"pr-reviews-`) {
		t.Fatalf("items by id: %d %s", status, got)
	}

	routes.check(t, []routeCase{
		{http.MethodDelete, "/api/schedules/" + view.ID, "", http.StatusOK, `"name":"pr-reviews"`},
		{http.MethodGet, "/api/schedules/pr-reviews", "", http.StatusNotFound, ""},
		// Its task stays.
		{http.MethodGet, "/api/tasks", "", http.StatusOK, `"title":"Review Fix the icon"`},
	})
}
