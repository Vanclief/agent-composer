package rest

import (
	"bytes"
	"context"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/vanclief/agent-composer/models/account"
)

// Made-up values: none is a real credential.
const (
	workValue = "made-up-work-value"
	homeValue = "made-up-home-value"
)

// ADR 0018: accounts are rows plus secrets.env lines, and no response
// ever carries a value.
func TestAccountRoutes(t *testing.T) {
	secretsPath := filepath.Join(t.TempDir(), "secrets.env")
	err := os.WriteFile(secretsPath, []byte("GITHUB_TOKEN=ghp_value\nACCOUNT_TAKEN=by-hand\n"), 0o600)
	if err != nil {
		t.Fatal(err)
	}

	// Failed requests are logged with their body: the value must not be.
	logged := &lockedBuffer{}
	previous := log.Logger
	log.Logger = zerolog.New(logged)
	t.Cleanup(func() {
		log.Logger = previous
	})

	rt := &statefulRuntime{sb: &fakeSandbox{}, statuses: map[string]string{}}
	routes := newTaskRoutes(t, rt, secretsPath)

	check := func(cases []routeCase) {
		t.Helper()

		for _, tc := range cases {
			status, body := routes.do(tc.method, tc.path, tc.body)
			if status != tc.status {
				t.Fatalf("%s %s: status %d, body %s", tc.method, tc.path, status, body)
			}
			if !strings.Contains(body, tc.want) {
				t.Fatalf("%s %s: body %s does not contain %s", tc.method, tc.path, body, tc.want)
			}
			for _, value := range []string{workValue, homeValue, "by-hand"} {
				if strings.Contains(body, value) {
					t.Fatalf("%s %s: the response carries a value", tc.method, tc.path)
				}
			}
		}
	}

	check([]routeCase{
		{http.MethodGet, "/api/accounts", "", http.StatusOK, `{"accounts":[],"providers":[{"id":"anthropic","harnesses":["claude_code"],"variables":{"oauth":"CLAUDE_CODE_OAUTH_TOKEN","api_key":"ANTHROPIC_API_KEY"}},{"id":"openai","harnesses":["codex_cli"],"variables":{"api_key":"CODEX_API_KEY"}}]}`},
		{http.MethodPost, "/api/accounts", `{"name":"Work","provider":"anthropic","kind":"oauth","value":"` + workValue + `"}`, http.StatusBadRequest, "lowercase letters, digits and dashes"},
		{http.MethodPost, "/api/accounts", `{"name":"work","provider":"google","kind":"oauth","value":"` + workValue + `"}`, http.StatusBadRequest, `unknown provider \"google\"`},
		{http.MethodPost, "/api/accounts", `{"name":"work","provider":"openai","kind":"oauth","value":"` + workValue + `"}`, http.StatusBadRequest, "provider openai has no oauth accounts yet"},
		{http.MethodPost, "/api/accounts", `{"name":"work","provider":"anthropic","kind":"oauth","value":" "}`, http.StatusBadRequest, "value is required"},
		{http.MethodPost, "/api/accounts", `{"name":"taken","provider":"anthropic","kind":"oauth","value":"` + workValue + `"}`, http.StatusConflict, "secret ACCOUNT_TAKEN is already set"},
		{http.MethodPost, "/api/accounts", `{"name":"work","provider":"anthropic","kind":"oauth","value":"` + workValue + `","email":"franco@example.com"}`, http.StatusCreated, `"name":"work","provider":"anthropic","kind":"oauth","secret":"ACCOUNT_WORK","email":"franco@example.com","default":false,"created_at":"20`},
		{http.MethodPost, "/api/accounts", `{"name":"work","provider":"openai","kind":"api_key","value":"` + homeValue + `"}`, http.StatusConflict, "account work already exists"},
		{http.MethodPost, "/api/accounts", `{"name":"home","provider":"anthropic","kind":"api_key","value":"` + homeValue + `"}`, http.StatusCreated, `"secret":"ACCOUNT_HOME","email":"","default":false`},
		{http.MethodGet, "/api/accounts", "", http.StatusOK, `"email":"franco@example.com","default":false,"created_at":"20`},
		{http.MethodGet, "/api/accounts", "", http.StatusOK, `"usage":null`},
	})

	info, err := os.Stat(secretsPath)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
		t.Fatalf("secrets.env mode = %04o", info.Mode().Perm())
	}
	data, err := os.ReadFile(secretsPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "ACCOUNT_WORK='"+workValue+"'\n") || !strings.HasPrefix(string(data), "GITHUB_TOKEN=ghp_value\nACCOUNT_TAKEN=by-hand\n") {
		t.Fatal("secrets.env does not hold the account line after the lines it had")
	}

	// One default per provider.
	check([]routeCase{
		{http.MethodPost, "/api/accounts/nobody/default", "", http.StatusNotFound, "account nobody not found"},
		{http.MethodPost, "/api/accounts/work/default", "", http.StatusOK, `"name":"work","provider":"anthropic","kind":"oauth","secret":"ACCOUNT_WORK","email":"franco@example.com","default":true`},
		{http.MethodPost, "/api/accounts/home/default", "", http.StatusOK, `"name":"home","provider":"anthropic","kind":"api_key","secret":"ACCOUNT_HOME","email":"","default":true`},
	})
	defaults := anthropicDefaults(t, routes)
	if len(defaults) != 1 || defaults[0] != "home" {
		t.Fatalf("anthropic defaults = %q", defaults)
	}

	check([]routeCase{
		{http.MethodDelete, "/api/accounts/home/default", "", http.StatusOK, `"name":"home","provider":"anthropic","kind":"api_key","secret":"ACCOUNT_HOME","email":"","default":false`},
	})
	if len(anthropicDefaults(t, routes)) != 0 {
		t.Fatal("clearing the default left one")
	}

	// A claude_code task with no credential in its spec gets the default,
	// which then cannot be deleted while the sandbox runs.
	check([]routeCase{
		{http.MethodPost, "/api/accounts/work/default", "", http.StatusOK, `"default":true`},
		{http.MethodPost, "/api/tasks", `{"title":"Use work","manager":"orchestrator","sandbox":{}}`, http.StatusOK, `"assignee":"claude_code"`},
	})
	routes.server.SandboxesAPI.Wait()

	check([]routeCase{
		{http.MethodGet, "/api/sandboxes/use-work", "", http.StatusOK, `"secrets":["ACCOUNT_WORK as CLAUDE_CODE_OAUTH_TOKEN"]`},
		{http.MethodDelete, "/api/accounts/work", "", http.StatusConflict, "account work is in use by running sandbox use-work"},
		{http.MethodPost, "/api/sandboxes/use-work/stop", "", http.StatusOK, `"status":"stopped"`},
		{http.MethodDelete, "/api/accounts/work", "", http.StatusNoContent, ""},
		{http.MethodDelete, "/api/accounts/work", "", http.StatusNotFound, "account work not found"},
		{http.MethodPost, "/api/accounts/work/default", "", http.StatusNotFound, "account work not found"},
	})

	data, err = os.ReadFile(secretsPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "ACCOUNT_WORK") || !strings.Contains(string(data), "ACCOUNT_HOME=") {
		t.Fatal("delete did not remove exactly the account's line")
	}

	_, err = account.GetByName(context.Background(), routes.db, "work")
	if err == nil {
		t.Fatal("the row is still there")
	}

	output := logged.String()
	if !strings.Contains(output, `"request_json":{"name":"Work"`) {
		t.Fatal("the failed create was not logged with its body, so this test checks nothing")
	}
	if strings.Contains(output, workValue) || strings.Contains(output, homeValue) {
		t.Fatal("a value reached the log")
	}
}

// lockedBuffer is a log sink safe for the boots' goroutines.
type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// anthropicDefaults lists the anthropic accounts marked default.
func anthropicDefaults(t *testing.T, routes *taskRoutes) []string {
	t.Helper()

	records, err := account.List(context.Background(), routes.db)
	if err != nil {
		t.Fatal(err)
	}

	names := []string{}
	for _, record := range records {
		if record.Provider == account.ProviderAnthropic && record.Default {
			names = append(names, record.Name)
		}
	}

	return names
}
