package usage

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// fakeToken is made up; the tests assert it never leaks into a result.
const fakeToken = "sk-ant-oat01-made-up-test-token-7f3a9c"

// usageResponse has the shape the Claude Code 2.1.316 bundle validates.
const usageResponse = `{
  "five_hour": {"utilization": 16, "resets_at": "2026-11-02T20:11:00.000+00:00"},
  "seven_day": {"utilization": 17.5, "resets_at": "2026-11-06T09:00:00.123Z"},
  "seven_day_oauth_apps": null,
  "seven_day_opus": {"utilization": 3, "resets_at": "2026-11-06T09:00:00Z"},
  "seven_day_sonnet": null,
  "extra_usage": {"is_enabled": false, "monthly_limit": null, "used_credits": null, "utilization": null},
  "limits": [
    {"kind": "weekly_scoped", "group": "weekly", "percent": 20, "resets_at": "2026-11-06T09:00:00Z",
     "severity": "normal", "is_active": false, "scope": {"model": {"display_name": "Fable"}}},
    {"kind": "spend", "group": "monthly", "percent": 54, "resets_at": "2026-12-01T00:00:00Z",
     "severity": "normal", "is_active": true}
  ]
}`

const profileResponse = `{
  "account": {"uuid": "a1", "email": "franco@example.com", "full_name": "F"},
  "organization": {"uuid": "o1"}
}`

type recorded struct {
	path    string
	headers http.Header
}

func newServer(t *testing.T, usage, profile func(w http.ResponseWriter)) (*httptest.Server, *[]recorded) {
	t.Helper()

	var seen []recorded
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, recorded{path: r.URL.Path, headers: r.Header.Clone()})
		switch r.URL.Path {
		case UsagePath:
			usage(w)
		case ProfilePath:
			profile(w)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)

	return srv, &seen
}

func body(s string) func(w http.ResponseWriter) {
	return func(w http.ResponseWriter) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(s))
	}
}

func status(code int) func(w http.ResponseWriter) {
	return func(w http.ResponseWriter) {
		w.WriteHeader(code)
		_, _ = w.Write([]byte(`{"error": {"type": "x"}}`))
	}
}

// assertNoToken fails when the token shows up anywhere in result.
func assertNoToken(t *testing.T, result Result, err error) {
	t.Helper()

	data, _ := json.Marshal(result)
	if strings.Contains(string(data), fakeToken) || strings.Contains(result.Email, fakeToken) {
		t.Fatalf("token leaked into result: %s", data)
	}
	if err != nil && strings.Contains(err.Error(), fakeToken) {
		t.Fatalf("token leaked into error: %v", err)
	}
}

func TestFetchMapsWindowsAndEmail(t *testing.T) {
	srv, seen := newServer(t, body(usageResponse), body(profileResponse))

	result, err := Fetcher{BaseURL: srv.URL}.Fetch(context.Background(), srv.Client(), fakeToken)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	assertNoToken(t, result, err)

	if result.Email != "franco@example.com" {
		t.Fatalf("email = %q", result.Email)
	}
	if result.Usage.Error != "" {
		t.Fatalf("error = %q", result.Usage.Error)
	}
	if result.Usage.UpdatedAt.IsZero() {
		t.Fatal("updated_at not set")
	}

	want := []Window{
		{ID: "session", Label: "Session", Used: 0.16, ResetsAt: "2026-11-02T20:11:00Z"},
		{ID: "weekly", Label: "Weekly", Used: 0.175, ResetsAt: "2026-11-06T09:00:00Z"},
		{ID: "opus", Label: "Opus", Used: 0.03, ResetsAt: "2026-11-06T09:00:00Z"},
		{ID: "fable", Label: "Fable", Used: 0.2, ResetsAt: "2026-11-06T09:00:00Z"},
	}
	got := result.Usage.Windows
	if len(got) != len(want) {
		t.Fatalf("windows = %+v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("window %d = %+v, want %+v", i, got[i], want[i])
		}
	}

	if len(*seen) != 2 {
		t.Fatalf("requests = %d", len(*seen))
	}
	for _, r := range *seen {
		if r.headers.Get("Authorization") != "Bearer "+fakeToken {
			t.Fatalf("%s: no bearer token", r.path)
		}
		if r.headers.Get("anthropic-beta") != OAuthBeta {
			t.Fatalf("%s: anthropic-beta = %q", r.path, r.headers.Get("anthropic-beta"))
		}
		if r.headers.Get("User-Agent") == "" {
			t.Fatalf("%s: no User-Agent", r.path)
		}
	}
}

func TestFetchRejectedToken(t *testing.T) {
	srv, _ := newServer(t, status(http.StatusUnauthorized), status(http.StatusUnauthorized))

	result, err := Fetcher{BaseURL: srv.URL}.Fetch(context.Background(), srv.Client(), fakeToken)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	assertNoToken(t, result, err)

	if result.Usage.Error != ErrTokenRejected {
		t.Fatalf("error = %q, want %q", result.Usage.Error, ErrTokenRejected)
	}
	if len(result.Usage.Windows) != 0 || result.Email != "" {
		t.Fatalf("result = %+v", result)
	}
}

func TestFetchServerError(t *testing.T) {
	srv, _ := newServer(t, status(http.StatusServiceUnavailable), body(profileResponse))

	result, err := Fetcher{BaseURL: srv.URL}.Fetch(context.Background(), srv.Client(), fakeToken)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	assertNoToken(t, result, err)

	if result.Usage.Error != "usage: HTTP 503" {
		t.Fatalf("error = %q", result.Usage.Error)
	}
	if result.Email != "franco@example.com" {
		t.Fatalf("email = %q, the profile still answered", result.Email)
	}
}

func TestFetchChangedShapeKeepsWhatItCanRead(t *testing.T) {
	srv, _ := newServer(t, body(`{"five_hour": {"utilization": 40, "resets_at": null}, "seven_day": "soon"}`), body(`[]`))

	result, err := Fetcher{BaseURL: srv.URL}.Fetch(context.Background(), srv.Client(), fakeToken)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	assertNoToken(t, result, err)

	if result.Usage.Error != "usage: unexpected response, profile: unexpected response" {
		t.Fatalf("error = %q", result.Usage.Error)
	}
	if len(result.Usage.Windows) != 1 || result.Usage.Windows[0].ID != "session" || result.Usage.Windows[0].ResetsAt != "" {
		t.Fatalf("windows = %+v", result.Usage.Windows)
	}
}

func TestFetchUnreachableNeverLeaksToken(t *testing.T) {
	srv, _ := newServer(t, body(usageResponse), body(profileResponse))
	srv.Close()

	result, err := Fetcher{BaseURL: srv.URL}.Fetch(context.Background(), nil, fakeToken)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	assertNoToken(t, result, err)

	if result.Usage.Error != "usage: unreachable, profile: unreachable" {
		t.Fatalf("error = %q", result.Usage.Error)
	}
}

func TestFetchNeedsToken(t *testing.T) {
	_, err := Fetch(context.Background(), nil, "")
	if err == nil {
		t.Fatal("want an error for an empty token")
	}
}

func TestFractionClamps(t *testing.T) {
	if fraction(130) != 1 || fraction(-5) != 0 || fraction(50) != 0.5 {
		t.Fatal("fraction does not clamp to 0..1")
	}
}
