package rest

import (
	"context"
	"database/sql"
	"encoding/json"
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
	"github.com/vanclief/agent-composer/core/events"
	"github.com/vanclief/agent-composer/core/resources/agents"
	"github.com/vanclief/agent-composer/core/resources/sandboxes"
	"github.com/vanclief/agent-composer/core/resources/schedules"
	"github.com/vanclief/agent-composer/core/resources/tasks"
	"github.com/vanclief/agent-composer/interfaces/rest/handler"
	restserver "github.com/vanclief/agent-composer/interfaces/rest/server"
	"github.com/vanclief/agent-composer/messaging"
	agentmodels "github.com/vanclief/agent-composer/models/agent"
	msgmodels "github.com/vanclief/agent-composer/models/messaging"
	schedulemodels "github.com/vanclief/agent-composer/models/schedule"
	taskmodels "github.com/vanclief/agent-composer/models/task"
	"github.com/vanclief/agent-composer/task"
)

// taskRoutes is the API served over an in-memory database.
type taskRoutes struct {
	e      *echo.Echo
	db     *bun.DB
	server *restserver.Server
	// volumes stands in for msb's named volume directory.
	volumes string
}

// newTaskRoutes serves the API routes over an in-memory database, with rt as
// the sandbox runtime and secretsPath as the secrets file.
func newTaskRoutes(t *testing.T, rt sandboxes.Runtime, secretsPath string) *taskRoutes {
	t.Helper()

	// The built-in defaults, with franco as the configured owner.
	t.Setenv("AGC_DEFAULT_HARNESS", "")
	t.Setenv("AGC_DEFAULT_MODEL", "")
	t.Setenv("AGC_DEFAULT_OWNER", "franco")

	return serveTaskRoutes(t, rt, secretsPath)
}

// serveTaskRoutes is newTaskRoutes with the AGC_DEFAULT_* environment left
// as the caller set it.
func serveTaskRoutes(t *testing.T, rt sandboxes.Runtime, secretsPath string) *taskRoutes {
	t.Helper()

	volumes := t.TempDir()
	t.Setenv(sandboxes.EnvVolumeRoot, volumes)

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
		(*taskmodels.Sandbox)(nil),
		(*msgmodels.Participant)(nil),
		(*msgmodels.Message)(nil),
		(*schedulemodels.Schedule)(nil),
		(*schedulemodels.Item)(nil),
		(*agentmodels.Agent)(nil),
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

	ctrl := &controller.Controller{DB: &relational.DB{DB: db}}
	bus := events.New()
	msg := messaging.NewService(db)
	msg.SetEvents(bus)
	specs := task.Store{Dir: filepath.Join(t.TempDir(), "specs")}
	sandboxesAPI := sandboxes.NewAPI(context.Background(), ctrl, msg, rt, secretsPath, socketDir)
	msg.SetPresence(sandboxesAPI)
	tasksAPI := tasks.NewAPI(ctrl, sandboxesAPI, msg, secretsPath, specs)
	tasksAPI.SetEvents(bus)
	schedulesAPI := schedules.NewAPI(ctrl, tasksAPI, secretsPath)
	schedulesAPI.SetEvents(bus)
	// The templates the repo ships, agents/ at its root.
	agentsAPI := agents.NewAPI(ctrl, tasksAPI, msg, filepath.Join("..", "..", "agents"))
	agentsAPI.SetEvents(bus)
	server := &restserver.Server{
		Ctrl:         ctrl,
		RateLimiter:  ratelimit.NewWindowCounter(10, 100),
		TasksAPI:     tasksAPI,
		SandboxesAPI: sandboxesAPI,
		SchedulesAPI: schedulesAPI,
		TurnsAPI:     agents.NewTurnsAPI(volumes, sandboxesAPI),
		AgentsAPI:    agentsAPI,
		Messaging:    msg,
		Events:       bus,
	}
	t.Cleanup(func() {
		schedulesAPI.Wait()
		agentsAPI.Wait()
		sandboxesAPI.Wait()
		_ = os.RemoveAll(socketDir)
	})

	e := echo.New()
	addAPIRoutes(e, handler.NewHandler(server))

	return &taskRoutes{e: e, db: db, server: server, volumes: volumes}
}

// do serves one request and returns its status and body.
func (r *taskRoutes) do(method, path, body string) (int, string) {
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	r.e.ServeHTTP(recorder, request)

	return recorder.Code, recorder.Body.String()
}

type routeCase struct {
	method string
	path   string
	body   string
	status int
	want   string
}

func (r *taskRoutes) check(t *testing.T, cases []routeCase) {
	t.Helper()

	for _, tc := range cases {
		status, body := r.do(tc.method, tc.path, tc.body)

		if status != tc.status {
			t.Fatalf("%s %s: status %d, body %s", tc.method, tc.path, status, body)
		}

		if !strings.Contains(body, tc.want) {
			t.Fatalf("%s %s: body %s does not contain %s", tc.method, tc.path, body, tc.want)
		}
	}
}

func TestTaskAndMessagingRoutes(t *testing.T) {
	routes := newTaskRoutes(t, sandboxes.SandboxRuntime{}, "")

	routes.check(t, []routeCase{
		{http.MethodGet, "/api/tasks", "", http.StatusOK, `{"tasks":[]}`},
		{http.MethodGet, "/api/sandboxes", "", http.StatusOK, `{"sandboxes":[]}`},
		{http.MethodGet, "/api/tasks/review-42", "", http.StatusNotFound, "task review-42 not found"},
		{http.MethodGet, "/api/tasks/0190a7c2-0000-7000-8000-000000000000", "", http.StatusNotFound, "task with ID"},
		{http.MethodGet, "/api/sandboxes/review-42", "", http.StatusNotFound, "sandbox review-42 not found"},
		{http.MethodPost, "/api/tasks/review-42/accept", "", http.StatusNotFound, "task review-42 not found"},
		{http.MethodPost, "/api/tasks/review-42/sandboxes", `{}`, http.StatusNotFound, "task review-42 not found"},
		{http.MethodPost, "/api/sandboxes/review-42/stop", "", http.StatusNotFound, "sandbox review-42 not found"},
		// The task-as-VM routes are gone (ADR 0008).
		{http.MethodPost, "/api/tasks/review-42/stop", "", http.StatusNotFound, "Not Found"},
		{http.MethodPost, "/api/tasks/review-42/start", "", http.StatusNotFound, "Not Found"},
		{http.MethodGet, "/api/tasks/review-42/terminals", "", http.StatusNotFound, "Not Found"},
		{http.MethodPost, "/api/tasks", `{"name":"Bad Name","brief":"b"}`, http.StatusBadRequest, ""},
		{http.MethodPost, "/api/tasks", `{"name":"a1b2c3d4-e5f6-4a7b-8c9d-0e1f2a3b4c5d","brief":"b"}`, http.StatusBadRequest, "looks like a UUID"},
		{http.MethodPost, "/api/msg/join", `{"name":"franco"}`, http.StatusOK, `"human":true`},
		{http.MethodGet, "/api/msg/channels", "", http.StatusOK, `"channels"`},
		{http.MethodGet, "/api/msg/participants?channel=%23review-42", "", http.StatusOK, `{"participants":[]}`},
		{http.MethodGet, "/api/msg/participants", "", http.StatusBadRequest, `"error"`},
	})
}

func TestCreateTaskReturnsAtOnceWithAStartingSandbox(t *testing.T) {
	sb := &fakeSandbox{}
	routes := newTaskRoutes(t, &fakeRuntime{sb: sb}, "")

	status, body := routes.do(http.MethodPost, "/api/tasks", `{"title":"Fix login","brief":"Make 2FA work.","assignee":"claude_code","manager":"orchestrator","for":"login-works","sandbox":{"repo":"github.com/o/n","cpus":2,"command":"claude --dangerously-skip-permissions"}}`)
	if status != http.StatusOK {
		t.Fatalf("status %d, body %s", status, body)
	}
	t.Logf("TaskView: %s", body)

	var view tasks.TaskView
	err := json.Unmarshal([]byte(body), &view)
	if err != nil {
		t.Fatal(err)
	}

	if view.Name != "fix-login" || view.Brief != "Make 2FA work." || view.Assignee != "claude_code" || view.For != "login-works" || view.Status != "open" {
		t.Fatalf("view = %+v", view)
	}
	if len(view.Sandboxes) != 1 {
		t.Fatalf("sandboxes = %+v", view.Sandboxes)
	}

	created := view.Sandboxes[0]
	if created.Name != "fix-login" || created.Task != "fix-login" || created.TaskID != view.ID || created.Status != "starting" {
		t.Fatalf("sandbox = %+v", created)
	}
	if created.Spec.Repo != "github.com/o/n" || created.Spec.CPUs != 2 || created.Command != "claude --dangerously-skip-permissions" {
		t.Fatalf("sandbox spec = %+v", created)
	}

	for _, key := range []string{`"done_at":null`, `"booted_at":null`, `"stopped_at":null`} {
		if !strings.Contains(body, key) {
			t.Fatalf("body %s lacks %s", body, key)
		}
	}
	if strings.Contains(body, `"status_error"`) {
		t.Fatalf("body %s has an empty status_error", body)
	}

	routes.server.SandboxesAPI.Wait()

	// The boot started the command as a terminal in the repo.
	if len(sb.started) != 1 || sb.started[0].Name != "claude" || sb.started[0].Dir != "/home/agent/n" {
		t.Fatalf("started %+v", sb.started)
	}

	routes.check(t, []routeCase{
		{http.MethodGet, "/api/tasks/fix-login", "", http.StatusOK, `"status":"stopped"`},
		{http.MethodGet, "/api/sandboxes/fix-login", "", http.StatusOK, `"task":"fix-login"`},
	})
}

func TestTaskAndSandboxRoutes(t *testing.T) {
	routes := newTaskRoutes(t, &fakeRuntime{sb: &fakeSandbox{}}, "")

	routes.check(t, []routeCase{
		{http.MethodPost, "/api/tasks", `{"title":"Review facebook ads","brief":"Find the ads that lose money.","manager":"franco"}`, http.StatusOK, `"title":"Review facebook ads","name":"review-facebook-ads","brief":"Find the ads that lose money."`},
		{http.MethodGet, "/api/tasks/review-facebook-ads", "", http.StatusOK, `"status":"open","done_at":null`},
		{http.MethodGet, "/api/tasks/review-facebook-ads", "", http.StatusOK, `"sandboxes":[]`},
		{http.MethodPost, "/api/tasks", `{"title":"No brief"}`, http.StatusBadRequest, "brief is required"},
		{http.MethodPost, "/api/tasks", `{"brief":"b"}`, http.StatusBadRequest, "title or name is required"},
		{http.MethodPost, "/api/tasks/review-facebook-ads/sandboxes", `{"ref":"master"}`, http.StatusBadRequest, "ref is set but repo is not"},
		{http.MethodPost, "/api/tasks/review-facebook-ads/sandboxes", `{"image":"debian"}`, http.StatusCreated, `"name":"review-facebook-ads","spec":{"image":"debian"`},
		{http.MethodPost, "/api/tasks/review-facebook-ads/sandboxes", `{"image":"debian","command":"bash"}`, http.StatusCreated, `"name":"review-facebook-ads-2"`},
	})

	routes.server.SandboxesAPI.Wait()

	status, body := routes.do(http.MethodGet, "/api/sandboxes", "")
	if status != http.StatusOK || strings.Index(body, `"name":"review-facebook-ads-2"`) > strings.Index(body, `"name":"review-facebook-ads"`) {
		t.Fatalf("list not newest first: %d %s", status, body)
	}

	routes.check(t, []routeCase{
		{http.MethodGet, "/api/tasks", "", http.StatusOK, `"name":"review-facebook-ads-2"`},
		{http.MethodGet, "/api/sandboxes/review-facebook-ads-2", "", http.StatusOK, `"command":"bash","instructions":"# Review facebook ads\n`},
		{http.MethodGet, "/api/sandboxes/review-facebook-ads-2", "", http.StatusOK, `"status":"stopped"`},
		{http.MethodPost, "/api/sandboxes/review-facebook-ads/stop", "", http.StatusOK, `"status":"stopped"`},
		{http.MethodPost, "/api/sandboxes/review-facebook-ads/start", "", http.StatusOK, `"stopped_at":null`},
		{http.MethodDelete, "/api/sandboxes/review-facebook-ads-2", "", http.StatusOK, `{"deleted":true,"name":"review-facebook-ads-2"}`},
		{http.MethodGet, "/api/sandboxes/review-facebook-ads-2", "", http.StatusNotFound, "not found"},
		{http.MethodPost, "/api/tasks/review-facebook-ads/accept", "", http.StatusOK, `"status":"done"`},
		{http.MethodGet, "/api/tasks/review-facebook-ads", "", http.StatusOK, `"status":"done","done_at":"20`},
		{http.MethodDelete, "/api/tasks/review-facebook-ads", "", http.StatusOK, `{"deleted":true,"name":"review-facebook-ads"}`},
		{http.MethodGet, "/api/sandboxes", "", http.StatusOK, `{"sandboxes":[]}`},
		{http.MethodGet, "/api/tasks", "", http.StatusOK, `{"tasks":[]}`},
	})
}

func TestSpecRoutes(t *testing.T) {
	routes := newTaskRoutes(t, &fakeRuntime{sb: &fakeSandbox{}}, "")

	routes.check(t, []routeCase{
		{http.MethodGet, "/api/specs", "", http.StatusOK, `{"specs":[]}`},
		{http.MethodPut, "/api/specs/Backend%20review", `{"yaml":"# mine\nrepo: github.com/o/n\n"}`, http.StatusOK, `{"name":"backend-review","yaml":"# mine\nrepo: github.com/o/n\n"}`},
		{http.MethodPut, "/api/specs/broken", `{"yaml":"ref: master\n"}`, http.StatusBadRequest, "ref is set but repo is not"},
		{http.MethodGet, "/api/specs", "", http.StatusOK, `{"specs":[{"name":"backend-review"`},
		{http.MethodGet, "/api/specs/backend-review", "", http.StatusOK, `"yaml":"# mine\n`},
		{http.MethodDelete, "/api/specs/backend-review", "", http.StatusOK, `"name":"backend-review"`},
		{http.MethodGet, "/api/specs/backend-review", "", http.StatusNotFound, "saved spec backend-review not found"},
		{http.MethodDelete, "/api/specs/backend-review", "", http.StatusNotFound, "saved spec backend-review not found"},
	})
}

func TestListSecretsRoute(t *testing.T) {
	secretsPath := filepath.Join(t.TempDir(), "secrets.env")
	err := os.WriteFile(secretsPath, []byte("GITHUB_TOKEN=ghp_routevalue\nCUSTOM=custom-routevalue\n"), 0o600)
	if err != nil {
		t.Fatal(err)
	}

	routes := newTaskRoutes(t, &fakeRuntime{sb: &fakeSandbox{}}, secretsPath)

	status, body := routes.do(http.MethodGet, "/api/secrets", "")
	if status != http.StatusOK {
		t.Fatalf("status %d, body %s", status, body)
	}

	want := `{"secrets":[{"name":"CUSTOM","hosts":[]},{"name":"GITHUB_TOKEN","hosts":["api.github.com","github.com"]}]}`
	body = strings.TrimSpace(body)
	if body != want {
		t.Fatalf("body %s, want %s", body, want)
	}

	if strings.Contains(body, "routevalue") {
		t.Fatalf("body %s contains a secret value", body)
	}
}
