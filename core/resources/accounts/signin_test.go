package accounts

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/vanclief/ez"

	"github.com/vanclief/agent-composer/core/resources/accounts/oauth"
	"github.com/vanclief/agent-composer/core/resources/accounts/usage"
	"github.com/vanclief/agent-composer/models/account"
)

// Made-up values: none is a real credential, and no test calls the real
// endpoints.
const (
	inferenceValue = "made-up-inference"
	pastedCode     = "made-up-code"
	firstAccess    = "made-up-access-1"
	firstRefresh   = "made-up-refresh-1"
	secondAccess   = "made-up-access-2"
	secondRefresh  = "made-up-refresh-2"
)

// fakeAnthropic is the token endpoint and the usage and profile
// endpoints. A refresh trades firstRefresh for the second pair unless
// refuse is set; usage and profile answer a login's access token, and
// refuse the inference value the way a `claude setup-token` token is
// refused.
type fakeAnthropic struct {
	t *testing.T

	mu      sync.Mutex
	refuse  bool
	grants  []map[string]string
	bearers []string
}

func (f *fakeAnthropic) serve(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()

	switch r.URL.Path {
	case "/v1/oauth/token":
		var body map[string]string
		err := json.NewDecoder(r.Body).Decode(&body)
		if err != nil {
			f.t.Error(err)
		}
		f.grants = append(f.grants, body)

		switch {
		case body["grant_type"] == "authorization_code" && body["code"] == pastedCode:
			_, _ = w.Write([]byte(`{"access_token":"` + firstAccess + `","refresh_token":"` + firstRefresh + `","expires_in":3600,"scope":"user:profile user:inference"}`))
		case body["grant_type"] == "refresh_token" && body["refresh_token"] == firstRefresh && !f.refuse:
			_, _ = w.Write([]byte(`{"access_token":"` + secondAccess + `","refresh_token":"` + secondRefresh + `","expires_in":3600}`))
		default:
			// The provider echoes what it got: none of it may surface.
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":"invalid_grant","code":"` + body["code"] + `","refresh_token":"` + body["refresh_token"] + `"}`))
		}
	case usage.UsagePath, usage.ProfilePath:
		token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		f.bearers = append(f.bearers, r.URL.Path+" "+token)

		if token == inferenceValue {
			if r.URL.Path == usage.ProfilePath {
				w.WriteHeader(http.StatusForbidden)
			} else {
				w.WriteHeader(http.StatusTooManyRequests)
			}
			return
		}
		if token != firstAccess && token != secondAccess {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		if r.URL.Path == usage.ProfilePath {
			_, _ = w.Write([]byte(`{"account":{"uuid":"u","email":"franco@example.com"},"organization":{"uuid":"o"}}`))
			return
		}
		_, _ = w.Write([]byte(`{"five_hour":{"utilization":25,"resets_at":"2026-11-02T21:00:00Z"}}`))
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

func (f *fakeAnthropic) lastGrant() map[string]string {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.grants[len(f.grants)-1]
}

// takeBearers returns and clears the endpoint calls since the last take.
func (f *fakeAnthropic) takeBearers() []string {
	f.mu.Lock()
	defer f.mu.Unlock()

	taken := f.bearers
	f.bearers = nil

	return taken
}

func (f *fakeAnthropic) setRefuse(refuse bool) {
	f.mu.Lock()
	f.refuse = refuse
	f.mu.Unlock()
}

// newSigninAPI is the accounts API and the usage poller connected as
// core.NewStack connects them, both talking to a fakeAnthropic.
func newSigninAPI(t *testing.T) (*API, *usage.Poller, *fakeAnthropic, string) {
	t.Helper()

	fake := &fakeAnthropic{t: t}
	srv := httptest.NewServer(http.HandlerFunc(fake.serve))
	t.Cleanup(srv.Close)

	api, secretsPath := newTestAPI(t, nil)
	cfg := oauth.Default()
	cfg.TokenURL = srv.URL + "/v1/oauth/token"
	api.SetOAuth(cfg, srv.Client())

	poller := usage.NewPoller(api, secretsPath)
	fetcher := usage.Fetcher{BaseURL: srv.URL}
	poller.Fetch = func(ctx context.Context, token string) (usage.Result, error) {
		return fetcher.Fetch(ctx, srv.Client(), token)
	}
	poller.SetLogins(api)
	source := PollerUsage{Poller: poller}
	api.SetUsage(source)
	api.SetPoller(source)

	_, err := api.Create(context.Background(), nil, &CreateRequest{Name: "work", Provider: account.ProviderAnthropic, Kind: account.KindOAuth, Value: inferenceValue})
	if err != nil {
		t.Fatal(err)
	}

	return api, poller, fake, secretsPath
}

// beginState begins a sign-in and returns its state, from the URL.
func beginState(t *testing.T, api *API) string {
	t.Helper()

	begun, err := api.Signin(context.Background(), nil, &SigninRequest{Name: "work"})
	if err != nil {
		t.Fatal(err)
	}

	parsed, err := url.Parse(begun.URL)
	if err != nil {
		t.Fatal(err)
	}

	return parsed.Query().Get("state")
}

func secretsFile(t *testing.T, path string) string {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	return string(data)
}

func TestSigninRefusals(t *testing.T) {
	ctx := context.Background()
	api, _, _, _ := newSigninAPI(t)

	_, err := api.Signin(ctx, nil, &SigninRequest{Name: "nobody"})
	if ez.ErrorCode(err) != ez.ENOTFOUND {
		t.Fatalf("unknown account: %v", err)
	}

	_, err = api.Create(ctx, nil, &CreateRequest{Name: "key", Provider: account.ProviderAnthropic, Kind: account.KindAPIKey, Value: inferenceValue})
	if err != nil {
		t.Fatal(err)
	}
	_, err = api.Signin(ctx, nil, &SigninRequest{Name: "key"})
	if ez.ErrorCode(err) != ez.EINVALID {
		t.Fatalf("an api_key account began a sign-in: %v", err)
	}

	_, err = api.SigninComplete(ctx, nil, &SigninCompleteRequest{Name: "work", Code: pastedCode})
	if ez.ErrorCode(err) != ez.EINVALID || ez.ErrorMessage(err) != errNothingPending {
		t.Fatalf("nothing pending: %v", err)
	}

	beginState(t, api)
	_, err = api.SigninComplete(ctx, nil, &SigninCompleteRequest{Name: "work", Code: pastedCode + "#another-state"})
	if ez.ErrorCode(err) != ez.EINVALID || !strings.Contains(ez.ErrorMessage(err), "another sign-in") {
		t.Fatalf("wrong state: %v", err)
	}

	// The provider refuses the code: its status, never its body.
	_, err = api.SigninComplete(ctx, nil, &SigninCompleteRequest{Name: "work", Code: "made-up-wrong-code"})
	if ez.ErrorCode(err) != ez.EINVALID || !strings.Contains(ez.ErrorMessage(err), "HTTP 400") || strings.Contains(err.Error(), "made-up-wrong-code") {
		t.Fatalf("refused exchange: %v", err)
	}

	if mustGet(t, api, "work").SignedInAt != nil {
		t.Fatal("a failed sign-in signed in")
	}

	// A request printed or logged hides its code.
	request := SigninCompleteRequest{Name: "work", Code: pastedCode}
	data, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), pastedCode) || strings.Contains(request.String(), pastedCode) {
		t.Fatal("the request shows its code")
	}
}

func mustGet(t *testing.T, api *API, name string) *account.Account {
	t.Helper()

	record, err := account.GetByName(context.Background(), api.db, name)
	if err != nil {
		t.Fatal(err)
	}

	return record
}

// Sign in, read usage with the login, refresh before expiry and rotate,
// expire, sign out back to the inference token, and delete.
func TestSigninLifecycle(t *testing.T) {
	ctx := context.Background()
	api, poller, fake, secretsPath := newSigninAPI(t)
	login := account.LoginSecretName("work")

	// Before signing in: the inference-only token is refused.
	poller.Poll(ctx)
	if poller.Usage("work").Error != usage.ErrInferenceOnly {
		t.Fatalf("inference token: %q", poller.Usage("work").Error)
	}
	fake.takeBearers()

	// A bare code is taken when one sign-in is pending.
	state := beginState(t, api)
	view, err := api.SigninComplete(ctx, nil, &SigninCompleteRequest{Name: "work", Code: pastedCode})
	if err != nil {
		t.Fatal(err)
	}

	grant := fake.lastGrant()
	if grant["grant_type"] != "authorization_code" || grant["state"] != state || grant["code"] != pastedCode || grant["code_verifier"] == "" {
		t.Fatal("the exchange did not send the pending state and verifier")
	}
	if view.SignedInAt == nil || view.Email != "franco@example.com" {
		t.Fatalf("view = %+v", view)
	}
	if view.Usage == nil || view.Usage.Error != "" || len(view.Usage.Windows) != 1 {
		t.Fatalf("usage = %+v", view.Usage)
	}
	if !strings.Contains(secretsFile(t, secretsPath), login+"='"+firstRefresh+"'\n") {
		t.Fatal("the refresh token is not the login line")
	}
	bearers := fake.takeBearers()
	if len(bearers) != 2 || bearers[0] != usage.UsagePath+" "+firstAccess || bearers[1] != usage.ProfilePath+" "+firstAccess {
		t.Fatal("the sign-in poll did not read both endpoints with the login's access token")
	}
	if mustGet(t, api, "work").Email != "franco@example.com" {
		t.Fatal("the row's email was not set")
	}

	// The pending sign-in is used up.
	_, err = api.SigninComplete(ctx, nil, &SigninCompleteRequest{Name: "work", Code: pastedCode + "#" + state})
	if ez.ErrorMessage(err) != errNothingPending {
		t.Fatalf("a used sign-in completed again: %v", err)
	}

	// A token that is still fresh is reused.
	poller.Poll(ctx)
	bearers = fake.takeBearers()
	if len(bearers) != 2 || !strings.HasSuffix(bearers[0], firstAccess) {
		t.Fatal("a fresh access token was not reused")
	}

	// Within five minutes of expiry: refreshed, and the rotated refresh
	// token replaces the line.
	api.loginMu.Lock()
	token := api.access["work"]
	token.ExpiresAt = time.Now().Add(4 * time.Minute)
	api.access["work"] = token
	api.loginMu.Unlock()

	poller.Poll(ctx)
	grant = fake.lastGrant()
	if grant["grant_type"] != "refresh_token" || grant["refresh_token"] != firstRefresh || grant["scope"] != "user:profile user:inference" {
		t.Fatal("no refresh with the stored refresh token")
	}
	bearers = fake.takeBearers()
	if len(bearers) != 2 || !strings.HasSuffix(bearers[0], secondAccess) || !strings.HasSuffix(bearers[1], secondAccess) {
		t.Fatal("usage was not read with the refreshed token")
	}
	file := secretsFile(t, secretsPath)
	if !strings.Contains(file, login+"='"+secondRefresh+"'\n") || strings.Contains(file, firstRefresh) {
		t.Fatal("the rotated refresh token was not written back")
	}

	// A refused refresh: sign-in expired, nothing else changes.
	fake.setRefuse(true)
	api.loginMu.Lock()
	delete(api.access, "work")
	api.loginMu.Unlock()

	poller.Poll(ctx)
	reading := poller.Usage("work")
	if reading.Error != usage.ErrSignInExpired || len(reading.Windows) != 1 {
		t.Fatalf("refused refresh: %+v", reading)
	}
	if !strings.Contains(secretsFile(t, secretsPath), login+"=") || mustGet(t, api, "work").SignedInAt == nil {
		t.Fatal("a refused refresh changed the login")
	}
	fake.takeBearers()

	// Sign out: the line goes, signed_in_at clears, usage is read with
	// the inference token again.
	view, err = api.Signout(ctx, nil, &SignoutRequest{Name: "work"})
	if err != nil {
		t.Fatal(err)
	}
	if view.SignedInAt != nil || mustGet(t, api, "work").SignedInAt != nil {
		t.Fatal("signed_in_at was not cleared")
	}
	file = secretsFile(t, secretsPath)
	if strings.Contains(file, login) || !strings.Contains(file, "ACCOUNT_WORK=") {
		t.Fatal("sign-out did not remove exactly the login line")
	}
	if view.Usage == nil || view.Usage.Error != usage.ErrInferenceOnly {
		t.Fatalf("after sign-out: %+v", view.Usage)
	}
	api.loginMu.Lock()
	_, kept := api.access["work"]
	api.loginMu.Unlock()
	if kept {
		t.Fatal("sign-out kept the access token")
	}

	// Sign in again, then delete the account: both lines go.
	fake.setRefuse(false)
	state = beginState(t, api)
	_, err = api.SigninComplete(ctx, nil, &SigninCompleteRequest{Name: "work", Code: pastedCode + "#" + state})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(secretsFile(t, secretsPath), login+"=") {
		t.Fatal("no login line after signing in again")
	}

	_, err = api.Delete(ctx, nil, &DeleteRequest{Name: "work"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(secretsFile(t, secretsPath), "ACCOUNT_WORK") {
		t.Fatal("delete left a line of the account")
	}
}

// An account whose secret is another's login line name blocks that
// sign-in, and is never removed as a login.
func TestSigninLineTakenByAnotherAccount(t *testing.T) {
	ctx := context.Background()
	api, _, _, secretsPath := newSigninAPI(t)

	_, err := api.Create(ctx, nil, &CreateRequest{Name: "work-login", Provider: account.ProviderAnthropic, Kind: account.KindOAuth, Value: "made-up-other"})
	if err != nil {
		t.Fatal(err)
	}

	state := beginState(t, api)
	_, err = api.SigninComplete(ctx, nil, &SigninCompleteRequest{Name: "work", Code: pastedCode + "#" + state})
	if ez.ErrorCode(err) != ez.ECONFLICT {
		t.Fatalf("sign-in over another account's line: %v", err)
	}

	_, err = api.Signout(ctx, nil, &SignoutRequest{Name: "work"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = api.Delete(ctx, nil, &DeleteRequest{Name: "work"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(secretsFile(t, secretsPath), "ACCOUNT_WORK_LOGIN='made-up-other'") {
		t.Fatal("another account's line was removed as a login")
	}
}
