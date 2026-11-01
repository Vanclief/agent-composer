package rest

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/sqlitedialect"
	"github.com/vanclief/compose/components/ratelimit"
	"github.com/vanclief/compose/drivers/databases/relational"
	_ "modernc.org/sqlite"

	"github.com/vanclief/agent-composer/core/controller"
	"github.com/vanclief/agent-composer/core/resources/tasks"
	"github.com/vanclief/agent-composer/interfaces/rest/handler"
	restserver "github.com/vanclief/agent-composer/interfaces/rest/server"
	"github.com/vanclief/agent-composer/messaging"
	msgmodels "github.com/vanclief/agent-composer/models/messaging"
	taskmodels "github.com/vanclief/agent-composer/models/task"
	"github.com/vanclief/agent-composer/task"
)

// newTaskRoutes serves the API routes over an in-memory database, with rt as
// the Tasks runtime.
func newTaskRoutes(t *testing.T, rt tasks.Runtime) (*echo.Echo, *bun.DB) {
	t.Helper()

	sqldb, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	sqldb.SetMaxOpenConns(1)
	t.Cleanup(func() {
		_ = sqldb.Close()
	})

	db := bun.NewDB(sqldb, sqlitedialect.New())
	tables := []interface{}{
		(*taskmodels.Task)(nil),
		(*msgmodels.Participant)(nil),
		(*msgmodels.Message)(nil),
	}
	for _, table := range tables {
		_, err = db.NewCreateTable().Model(table).Exec(context.Background())
		if err != nil {
			t.Fatal(err)
		}
	}

	// Unix socket paths are capped near 104 bytes, too short for
	// t.TempDir() on macOS.
	socketDir, err := os.MkdirTemp("", "agc")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.RemoveAll(socketDir)
	})

	ctrl := &controller.Controller{DB: &relational.DB{DB: db}}
	msg := messaging.NewService(db)
	specs := task.Store{Dir: filepath.Join(t.TempDir(), "specs")}
	server := &restserver.Server{
		Ctrl:        ctrl,
		RateLimiter: ratelimit.NewWindowCounter(10, 100),
		TasksAPI:    tasks.NewAPI(ctrl, msg, rt, "", socketDir, specs),
		Messaging:   msg,
	}

	e := echo.New()
	addAPIRoutes(e, handler.NewHandler(server))

	return e, db
}

func TestTaskAndMessagingRoutes(t *testing.T) {
	e, _ := newTaskRoutes(t, tasks.SandboxRuntime{})

	cases := []struct {
		method string
		path   string
		body   string
		status int
		want   string
	}{
		{http.MethodGet, "/api/tasks", "", http.StatusOK, `{"tasks":[]}`},
		{http.MethodGet, "/api/tasks/review-42", "", http.StatusNotFound, "task review-42 not found"},
		{http.MethodGet, "/api/tasks/0190a7c2-0000-7000-8000-000000000000", "", http.StatusNotFound, "task with ID"},
		{http.MethodPost, "/api/tasks/review-42/stop", "", http.StatusNotFound, "task review-42 not found"},
		{http.MethodPost, "/api/tasks", `{"name":"Bad Name","spec":{}}`, http.StatusBadRequest, ""},
		{http.MethodPost, "/api/tasks", `{"name":"a1b2c3d4-e5f6-4a7b-8c9d-0e1f2a3b4c5d","spec":{}}`, http.StatusBadRequest, "looks like a UUID"},
		{http.MethodPost, "/api/msg/join", `{"name":"franco"}`, http.StatusOK, `"human":true`},
		{http.MethodGet, "/api/msg/channels", "", http.StatusOK, `"channels"`},
	}

	for _, tc := range cases {
		request := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
		request.Header.Set("Content-Type", "application/json")
		recorder := httptest.NewRecorder()

		e.ServeHTTP(recorder, request)

		if recorder.Code != tc.status {
			t.Fatalf("%s %s: status %d, body %s", tc.method, tc.path, recorder.Code, recorder.Body.String())
		}

		if !strings.Contains(recorder.Body.String(), tc.want) {
			t.Fatalf("%s %s: body %s does not contain %s", tc.method, tc.path, recorder.Body.String(), tc.want)
		}
	}
}

func TestCreateTaskWithTitleAndSpecRoutes(t *testing.T) {
	e, _ := newTaskRoutes(t, &fakeRuntime{sb: &fakeSandbox{}})

	cases := []struct {
		method string
		path   string
		body   string
		status int
		want   string
	}{
		{http.MethodPost, "/api/tasks", `{"title":"Review facebook ads","spec":{}}`, http.StatusOK, `"title":"Review facebook ads","name":"review-facebook-ads"`},
		{http.MethodGet, "/api/tasks/review-facebook-ads", "", http.StatusOK, `"title":"Review facebook ads"`},
		{http.MethodDelete, "/api/tasks/review-facebook-ads", "", http.StatusOK, `"deleted":true`},
		{http.MethodPost, "/api/tasks", `{"spec":{}}`, http.StatusBadRequest, "title or name is required"},
		{http.MethodGet, "/api/specs", "", http.StatusOK, `{"specs":[]}`},
		{http.MethodPut, "/api/specs/Backend%20review", `{"yaml":"# mine\nrepo: github.com/o/n\n"}`, http.StatusOK, `{"name":"backend-review","yaml":"# mine\nrepo: github.com/o/n\n"}`},
		{http.MethodPut, "/api/specs/broken", `{"yaml":"ref: master\n"}`, http.StatusBadRequest, "ref is set but repo is not"},
		{http.MethodGet, "/api/specs", "", http.StatusOK, `{"specs":[{"name":"backend-review"`},
		{http.MethodGet, "/api/specs/backend-review", "", http.StatusOK, `"yaml":"# mine\n`},
		{http.MethodDelete, "/api/specs/backend-review", "", http.StatusOK, `"name":"backend-review"`},
		{http.MethodGet, "/api/specs/backend-review", "", http.StatusNotFound, "saved spec backend-review not found"},
		{http.MethodDelete, "/api/specs/backend-review", "", http.StatusNotFound, "saved spec backend-review not found"},
	}

	for _, tc := range cases {
		request := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
		request.Header.Set("Content-Type", "application/json")
		recorder := httptest.NewRecorder()

		e.ServeHTTP(recorder, request)

		if recorder.Code != tc.status {
			t.Fatalf("%s %s: status %d, body %s", tc.method, tc.path, recorder.Code, recorder.Body.String())
		}

		if !strings.Contains(recorder.Body.String(), tc.want) {
			t.Fatalf("%s %s: body %s does not contain %s", tc.method, tc.path, recorder.Body.String(), tc.want)
		}
	}
}
