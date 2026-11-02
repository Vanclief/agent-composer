package rest

import (
	"net/http"
	"testing"
)

// GET /api/defaults shows the environment over the built-ins, and agc's
// own messaging name.
func TestDefaultsRoute(t *testing.T) {
	t.Setenv("AGC_DEFAULT_HARNESS", "codex_cli")
	t.Setenv("AGC_DEFAULT_MODEL", "gpt-5.5")
	t.Setenv("AGC_DEFAULT_OWNER", "ana")
	routes := serveTaskRoutes(t, &fakeRuntime{sb: &fakeSandbox{}}, "")

	routes.check(t, []routeCase{
		{http.MethodGet, "/api/defaults", "", http.StatusOK, `{"harness":"codex_cli","model":"gpt-5.5","owner":"ana","system":"agc"}`},
	})
}

// Unset, harness and model fall back to the built-ins and the owner is "".
func TestDefaultsRouteUnset(t *testing.T) {
	t.Setenv("AGC_DEFAULT_HARNESS", "")
	t.Setenv("AGC_DEFAULT_MODEL", "")
	t.Setenv("AGC_DEFAULT_OWNER", "")
	routes := serveTaskRoutes(t, &fakeRuntime{sb: &fakeSandbox{}}, "")

	routes.check(t, []routeCase{
		{http.MethodGet, "/api/defaults", "", http.StatusOK, `{"harness":"claude_code","model":"claude-opus-5-5","owner":"","system":"agc"}`},
	})
}
