package rest

import (
	"context"
	"net/http"
	"testing"
	"time"

	taskmodels "github.com/vanclief/agent-composer/models/task"
)

// ADR 0008 "Garbage collection": POST /api/sandboxes/gc lists, then
// deletes, the stopped sandboxes of tasks done longer than keep ago.
func TestSandboxGCRoutes(t *testing.T) {
	rt := &homeRuntime{fakeRuntime: fakeRuntime{sb: &fakeSandbox{}}, statuses: map[string]string{}}
	routes := newTaskRoutes(t, rt, "")
	ctx := context.Background()

	booted := time.Now().Add(-72 * time.Hour)
	longAgo := time.Now().Add(-48 * time.Hour)
	done := &taskmodels.Task{Title: "Done", Name: "done-task", Brief: "b", DoneAt: &longAgo}
	open := &taskmodels.Task{Title: "Open", Name: "open-task", Brief: "b"}
	for _, owner := range []*taskmodels.Task{done, open} {
		err := owner.Insert(ctx, routes.db)
		if err != nil {
			t.Fatal(err)
		}
		sb := &taskmodels.Sandbox{TaskID: owner.ID, Name: owner.Name, Spec: []byte(`{"disk":"30G"}`), Volume: "agc-" + owner.Name, BootedAt: &booted}
		err = sb.Insert(ctx, routes.db)
		if err != nil {
			t.Fatal(err)
		}
		rt.set(owner.Name, "stopped")
	}

	routes.check(t, []routeCase{
		{http.MethodGet, "/api/sandboxes", "", http.StatusOK, `"disk":"30G","disk_mib":30720`},
		{http.MethodPost, "/api/sandboxes/gc", `{"keep":"soon"}`, http.StatusBadRequest, "is not a duration"},
		{http.MethodPost, "/api/sandboxes/gc", `{"dry_run":true}`, http.StatusOK,
			`{"dry_run":true,"keep":"24h0m0s","sandboxes":[{"name":"done-task","task":"done-task","done_for":"48h0m0s","disk":"30G","disk_mib":30720}],"freed":0,"freed_disk":"0G"}`},
		{http.MethodGet, "/api/sandboxes/done-task", "", http.StatusOK, `"name":"done-task"`},
		{http.MethodPost, "/api/sandboxes/gc", `{"keep":"72h"}`, http.StatusOK, `"sandboxes":[],"freed":0`},
		{http.MethodPost, "/api/sandboxes/gc", "", http.StatusOK, `"freed":1,"freed_disk":"30G"`},
		{http.MethodGet, "/api/sandboxes/done-task", "", http.StatusNotFound, "not found"},
		{http.MethodGet, "/api/sandboxes/open-task", "", http.StatusOK, `"status":"stopped"`},
	})
}
