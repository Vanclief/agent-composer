package rest

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/vanclief/agent-composer/core/resources/accounts"
	"github.com/vanclief/agent-composer/core/resources/accounts/oauth"
	"github.com/vanclief/agent-composer/core/resources/accounts/usage"
	"github.com/vanclief/agent-composer/models/account"
)

// Made-up values: none is a real credential, and the test never calls
// the real endpoints.
const (
	signinInference = "made-up-inference-value"
	signinCode      = "made-up-pasted-code"
	signinAccess    = "made-up-access-token"
	signinRefresh   = "made-up-refresh-token"
)

// ADR 0018 "Sign-in": begin → URL, complete with the pasted code#state
// against an httptest token server, sign out, delete; no token or code in
// any response or log line.
func TestAccountSigninRoutes(t *testing.T) {
	secretsPath := filepath.Join(t.TempDir(), "secrets.env")

	logged := &lockedBuffer{}
	previous := log.Logger
	log.Logger = zerolog.New(logged)
	t.Cleanup(func() {
		log.Logger = previous
	})

	// The token endpoint and the usage and profile endpoints.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/oauth/token":
			var body map[string]string
			err := json.NewDecoder(r.Body).Decode(&body)
			if err != nil || body["code"] != signinCode {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			_, _ = w.Write([]byte(`{"access_token":"` + signinAccess + `","refresh_token":"` + signinRefresh + `","expires_in":3600}`))
		case usage.ProfilePath:
			if r.Header.Get("Authorization") != "Bearer "+signinAccess {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			_, _ = w.Write([]byte(`{"account":{"uuid":"u","email":"franco@example.com"}}`))
		case usage.UsagePath:
			if r.Header.Get("Authorization") != "Bearer "+signinAccess {
				w.WriteHeader(http.StatusTooManyRequests)
				return
			}
			_, _ = w.Write([]byte(`{"seven_day":{"utilization":40}}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)

	rt := &statefulRuntime{sb: &fakeSandbox{}, statuses: map[string]string{}}
	routes := newTaskRoutes(t, rt, secretsPath)

	api := routes.server.AccountsAPI
	cfg := oauth.Default()
	cfg.TokenURL = srv.URL + "/v1/oauth/token"
	api.SetOAuth(cfg, srv.Client())

	poller := usage.NewPoller(api, secretsPath)
	fetcher := usage.Fetcher{BaseURL: srv.URL}
	poller.Fetch = func(ctx context.Context, token string) (usage.Result, error) {
		return fetcher.Fetch(ctx, srv.Client(), token)
	}
	poller.SetLogins(api)
	api.SetUsage(accounts.PollerUsage{Poller: poller})
	api.SetPoller(accounts.PollerUsage{Poller: poller})

	responses := []string{}
	check := func(cases []routeCase) {
		t.Helper()

		for _, tc := range cases {
			status, body := routes.do(tc.method, tc.path, tc.body)
			responses = append(responses, body)
			if status != tc.status {
				t.Fatalf("%s %s: status %d, body %s", tc.method, tc.path, status, body)
			}
			if !strings.Contains(body, tc.want) {
				t.Fatalf("%s %s: body %s does not contain %s", tc.method, tc.path, body, tc.want)
			}
		}
	}

	begin := func() string {
		t.Helper()

		status, body := routes.do(http.MethodPost, "/api/accounts/work/signin", "")
		responses = append(responses, body)
		if status != http.StatusOK {
			t.Fatalf("begin: status %d, body %s", status, body)
		}

		var begun accounts.SigninResponse
		err := json.Unmarshal([]byte(body), &begun)
		if err != nil {
			t.Fatal(err)
		}
		if begun.ExpiresAt.IsZero() || !strings.Contains(body, `"expires_at":"20`) {
			t.Fatalf("begin: %s", body)
		}

		parsed, err := url.Parse(begun.URL)
		if err != nil {
			t.Fatal(err)
		}
		query := parsed.Query()
		if parsed.Host != "claude.com" || query.Get("code_challenge_method") != "S256" || query.Get("code_challenge") == "" || query.Get("redirect_uri") != oauth.DefaultRedirectURL {
			t.Fatalf("authorize URL %s", begun.URL)
		}

		return query.Get("state")
	}

	check([]routeCase{
		{http.MethodPost, "/api/accounts", `{"name":"work","provider":"anthropic","kind":"oauth","value":"` + signinInference + `"}`, http.StatusCreated, `"signed_in_at":null`},
		{http.MethodPost, "/api/accounts/nobody/signin", "", http.StatusNotFound, "account nobody not found"},
		{http.MethodPost, "/api/accounts/work/signin/complete", `{"code":"` + signinCode + `"}`, http.StatusBadRequest, "no sign-in pending for this account; start again"},
	})

	state := begin()

	check([]routeCase{
		{http.MethodPost, "/api/accounts/work/signin/complete", `{"code":"` + signinCode + `#not-the-state"}`, http.StatusBadRequest, "start again"},
		{http.MethodPost, "/api/accounts/work/signin/complete", `{"code":"made-up-wrong#` + state + `"}`, http.StatusBadRequest, "token endpoint answered HTTP 400"},
		{http.MethodPost, "/api/accounts/work/signin/complete", `{"code":" "}`, http.StatusBadRequest, "code is required"},
		{http.MethodPost, "/api/accounts/work/signin/complete", `{"code":"` + signinCode + `#` + state + `"}`, http.StatusOK, `"email":"franco@example.com"`},
		{http.MethodGet, "/api/accounts", "", http.StatusOK, `"signed_in_at":"20`},
		{http.MethodGet, "/api/accounts", "", http.StatusOK, `"id":"weekly","label":"Weekly","used":0.4`},
	})

	login := account.LoginSecretName("work")
	data, err := os.ReadFile(secretsPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), login+"='"+signinRefresh+"'\n") {
		t.Fatal("the login line was not written")
	}

	record, err := account.GetByName(context.Background(), routes.db, "work")
	if err != nil {
		t.Fatal(err)
	}
	if record.SignedInAt == nil || record.Email != "franco@example.com" {
		t.Fatalf("row = %+v", record)
	}

	check([]routeCase{
		{http.MethodDelete, "/api/accounts/work/signin", "", http.StatusOK, `"signed_in_at":null`},
		{http.MethodGet, "/api/accounts", "", http.StatusOK, `"error":"inference-only token: sign in for usage"`},
	})

	data, err = os.ReadFile(secretsPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), login) || !strings.Contains(string(data), "ACCOUNT_WORK=") {
		t.Fatal("sign-out did not remove exactly the login line")
	}

	// Signed in again, deleting the account removes the login line too.
	state = begin()
	check([]routeCase{
		{http.MethodPost, "/api/accounts/work/signin/complete", `{"code":"` + signinCode + `#` + state + `"}`, http.StatusOK, `"signed_in_at":"20`},
		{http.MethodDelete, "/api/accounts/work", "", http.StatusNoContent, ""},
		{http.MethodDelete, "/api/accounts/work/signin", "", http.StatusNotFound, "account work not found"},
	})

	data, err = os.ReadFile(secretsPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "ACCOUNT_WORK") {
		t.Fatal("delete left a line of the account")
	}

	output := logged.String()
	if !strings.Contains(output, `"request_json":{"name":"work"}`) {
		t.Fatal("the failed completes were not logged with their body, so this test checks nothing")
	}
	for _, secret := range []string{signinCode, signinAccess, signinRefresh, signinInference, "made-up-wrong"} {
		for _, body := range responses {
			if strings.Contains(body, secret) {
				t.Fatalf("a response carries a token or the code: %s", body)
			}
		}
		if strings.Contains(output, secret) {
			t.Fatal("a token or the code reached the log")
		}
	}
}
