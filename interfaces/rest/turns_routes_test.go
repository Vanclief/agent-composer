package rest

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/vanclief/agent-composer/core/resources/sandboxes"
	taskmodels "github.com/vanclief/agent-composer/models/task"
)

func readFixture(t *testing.T, name string) string {
	t.Helper()

	body, err := os.ReadFile(filepath.Join("..", "..", "core", "resources", "agents", "testdata", name))
	if err != nil {
		t.Fatal(err)
	}

	return string(body)
}

// The same session files serve an agent's volume (agc-<name>, no row
// needed) and a task sandbox's.
func TestTurnsRoutes(t *testing.T) {
	routes := newTaskRoutes(t, sandboxes.SandboxRuntime{}, "")

	owner := &taskmodels.Task{Title: "Review 42", Name: "review-42", Brief: "b"}
	err := owner.Insert(context.Background(), routes.db)
	if err != nil {
		t.Fatal(err)
	}

	sb := &taskmodels.Sandbox{TaskID: owner.ID, Name: "review-42", Spec: []byte(`{}`), Volume: "agc-review-42"}
	err = sb.Insert(context.Background(), routes.db)
	if err != nil {
		t.Fatal(err)
	}

	modified := time.Date(2026, 11, 2, 12, 0, 0, 0, time.UTC)
	claude := readFixture(t, "claude-two-turns.jsonl")
	codex := readFixture(t, "codex-one-turn.jsonl")
	for _, volume := range []string{"agc-reviewer-1", "agc-review-42"} {
		dir := filepath.Join(routes.volumes, volume)
		writeVolumeFile(t, dir, ".claude/projects/-home-agent-project/11111111-2222-4333-8444-555555555555.jsonl", claude, modified)
		writeVolumeFile(t, dir, ".codex/sessions/2026/11/02/rollout-2026-11-02T11-00-00-019a0000-0000-7000-8000-000000000001.jsonl", codex, modified)
	}
	writeVolumeFile(t, filepath.Join(routes.volumes, "agc-reviewer-1"), "agc-sessions.json", `{"#reviewer-1:root-1":"11111111-2222-4333-8444-555555555555"}`, modified)

	routes.check(t, []routeCase{
		{http.MethodGet, "/api/agents/reviewer-1/turns", "", http.StatusOK, `{"turns":[{"n":3,"started_at":"2026-11-02T11:00:02Z","ended_at":"2026-11-02T11:00:06Z","session":"019a0000-0000-7000-8000-000000000001","harness":"codex_cli","channel":"","thread":"","tool_calls":1,"answer":"hi","tokens":{"input":2100,"output":45,"cache_read":8000,"cache_write":0}},{"n":2,`},
		{http.MethodGet, "/api/agents/reviewer-1/turns", "", http.StatusOK, `"channel":"#reviewer-1","thread":"root-1","tool_calls":0,"answer":"0002-storage.md is newer."`},
		{http.MethodGet, "/api/agents/reviewer-1/turns", "", http.StatusOK, `"skipped":0}`},
		{http.MethodGet, "/api/agents/reviewer-1/turns?limit=1&before=3", "", http.StatusOK, `{"turns":[{"n":2,`},
		{http.MethodGet, "/api/agents/reviewer-1/turns/1", "", http.StatusOK, `"injected":"List the ADRs in docs/decisions.","tool_calls":[{"name":"Bash","args":{"command":"ls docs/decisions","description":"List ADRs"},"result":"0001-git.md\n0002-storage.md","truncated":false}],"answer":"There are two ADRs: 0001 and 0002.\nBoth are accepted."`},
		{http.MethodGet, "/api/agents/reviewer-1/turns/4", "", http.StatusNotFound, "turn 4 not found"},
		{http.MethodGet, "/api/agents/reviewer-1/turns/x", "", http.StatusBadRequest, "n must be a whole number"},
		{http.MethodGet, "/api/agents/reviewer-1/turns?limit=-1", "", http.StatusBadRequest, "limit must be"},
		{http.MethodGet, "/api/agents/nobody-9/turns", "", http.StatusNotFound, "has no volume agc-nobody-9"},
		{http.MethodGet, "/api/agents/nobody-9/turns/1", "", http.StatusNotFound, "has no volume agc-nobody-9"},
		{http.MethodGet, "/api/agents/..%2fagc-review-42/turns", "", http.StatusNotFound, ""},

		{http.MethodGet, "/api/sandboxes/review-42/turns", "", http.StatusOK, `{"turns":[{"n":3,`},
		{http.MethodGet, "/api/sandboxes/review-42/turns", "", http.StatusOK, `"channel":"","thread":"","tool_calls":0,"answer":"0002-storage.md is newer."`},
		{http.MethodGet, "/api/sandboxes/" + sb.ID.String() + "/turns/3", "", http.StatusOK, `"harness":"codex_cli"`},
		{http.MethodGet, "/api/sandboxes/review-42/turns/2", "", http.StatusOK, `"injected":"Which one is newer?"`},
		{http.MethodGet, "/api/sandboxes/never/turns", "", http.StatusNotFound, "no volume agc-never is left"},
	})
}
