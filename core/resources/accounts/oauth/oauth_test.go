package oauth

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/vanclief/ez"
)

// Made-up values: none is a real credential.
const (
	madeUpCode    = "made-up-code"
	madeUpAccess  = "made-up-access"
	madeUpRefresh = "made-up-refresh"
	madeUpRotated = "made-up-rotated"
)

func TestBeginBuildsTheAuthorizeURL(t *testing.T) {
	cfg := Default()
	pending, link := cfg.Begin()

	parsed, err := url.Parse(link)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Scheme+"://"+parsed.Host+parsed.Path != DefaultAuthorizeURL {
		t.Fatalf("authorize URL = %s", link)
	}

	query := parsed.Query()
	want := map[string]string{
		"code":                  "true",
		"client_id":             DefaultClientID,
		"response_type":         "code",
		"redirect_uri":          DefaultRedirectURL,
		"scope":                 "user:profile user:inference",
		"code_challenge":        pending.Challenge,
		"code_challenge_method": "S256",
		"state":                 pending.State,
	}
	for key, value := range want {
		if query.Get(key) != value {
			t.Fatalf("%s = %q, want %q", key, query.Get(key), value)
		}
	}
	if len(query) != len(want) {
		t.Fatalf("query has %d keys, want %d", len(query), len(want))
	}

	if len(pending.Verifier) < 43 || len(pending.Verifier) > 128 {
		t.Fatalf("verifier is %d characters", len(pending.Verifier))
	}
	sum := sha256.Sum256([]byte(pending.Verifier))
	if base64.RawURLEncoding.EncodeToString(sum[:]) != query.Get("code_challenge") {
		t.Fatal("the challenge is not S256 of the verifier")
	}
	if pending.State == "" || strings.ContainsAny(pending.State, "+/=#") {
		t.Fatalf("state %q is not url-safe", pending.State)
	}
	if time.Until(pending.ExpiresAt) < 9*time.Minute {
		t.Fatalf("expires at %s", pending.ExpiresAt)
	}

	other, _ := cfg.Begin()
	if other.State == pending.State || other.Verifier == pending.Verifier {
		t.Fatal("two sign-ins share a state or a verifier")
	}
}

// tokenServer answers the token endpoint with status and answer, and
// records the JSON body it was sent.
func tokenServer(t *testing.T, status int, answer string, got *map[string]string) Config {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("%s with Content-Type %q", r.Method, r.Header.Get("Content-Type"))
		}
		err := json.NewDecoder(r.Body).Decode(got)
		if err != nil {
			t.Error(err)
		}
		w.WriteHeader(status)
		_, _ = w.Write([]byte(answer))
	}))
	t.Cleanup(srv.Close)

	cfg := Default()
	cfg.TokenURL = srv.URL + "/v1/oauth/token"

	return cfg
}

func TestExchange(t *testing.T) {
	var got map[string]string
	cfg := tokenServer(t, http.StatusOK, `{"access_token":"`+madeUpAccess+`","refresh_token":"`+madeUpRefresh+`","expires_in":3600,"scope":"user:profile user:inference"}`, &got)

	token, err := Exchange(context.Background(), nil, cfg, madeUpCode, "the-state", "the-verifier")
	if err != nil {
		t.Fatal(err)
	}

	want := map[string]string{
		"grant_type":    "authorization_code",
		"code":          madeUpCode,
		"state":         "the-state",
		"client_id":     DefaultClientID,
		"redirect_uri":  DefaultRedirectURL,
		"code_verifier": "the-verifier",
	}
	if len(got) != len(want) {
		t.Fatalf("body = %v", got)
	}
	for key, value := range want {
		if got[key] != value {
			t.Fatalf("%s = %q, want %q", key, got[key], value)
		}
	}

	if token.Access != madeUpAccess || token.Refresh != madeUpRefresh || token.Scope != "user:profile user:inference" {
		t.Fatalf("token = %+v", token.Scope)
	}
	left := time.Until(token.ExpiresAt)
	if left < 59*time.Minute || left > time.Hour {
		t.Fatalf("expires in %s", left)
	}
}

func TestRefresh(t *testing.T) {
	var got map[string]string
	cfg := tokenServer(t, http.StatusOK, `{"access_token":"`+madeUpAccess+`","refresh_token":"`+madeUpRotated+`","expires_in":28800}`, &got)

	token, err := Refresh(context.Background(), nil, cfg, madeUpRefresh)
	if err != nil {
		t.Fatal(err)
	}

	want := map[string]string{
		"grant_type":    "refresh_token",
		"refresh_token": madeUpRefresh,
		"client_id":     DefaultClientID,
		"scope":         "user:profile user:inference",
	}
	if len(got) != len(want) {
		t.Fatalf("body has %d keys", len(got))
	}
	for key, value := range want {
		if got[key] != value {
			t.Fatalf("%s is not what Claude Code sends", key)
		}
	}
	if token.Access != madeUpAccess || token.Refresh != madeUpRotated {
		t.Fatal("the response was not parsed")
	}

	// No refresh_token back: the old one stays.
	cfg = tokenServer(t, http.StatusOK, `{"access_token":"`+madeUpAccess+`","expires_in":60}`, &got)
	token, err = Refresh(context.Background(), nil, cfg, madeUpRefresh)
	if err != nil {
		t.Fatal(err)
	}
	if token.Refresh != madeUpRefresh {
		t.Fatal("a refresh without a new refresh token lost the old one")
	}
}

func TestRefreshRefused(t *testing.T) {
	var got map[string]string
	// The provider's body echoes what it was sent: none of it may reach
	// the error.
	cfg := tokenServer(t, http.StatusBadRequest, `{"error":"invalid_grant","refresh_token":"`+madeUpRefresh+`","access_token":"`+madeUpAccess+`"}`, &got)

	_, err := Refresh(context.Background(), nil, cfg, madeUpRefresh)
	if err == nil {
		t.Fatal("a 400 refreshed")
	}

	wrapped := ez.Wrap(err)
	if Status(wrapped) != http.StatusBadRequest || !Expired(wrapped) {
		t.Fatalf("status = %d", Status(wrapped))
	}

	text := err.Error() + " " + ez.ErrorMessage(err)
	if !strings.Contains(text, "400") {
		t.Fatalf("error %q does not hold the status", text)
	}
	for _, secret := range []string{madeUpRefresh, madeUpAccess, "invalid_grant"} {
		if strings.Contains(text, secret) {
			t.Fatal("the error holds the provider's body")
		}
	}

	cfg = tokenServer(t, http.StatusInternalServerError, `{}`, &got)
	_, err = Refresh(context.Background(), nil, cfg, madeUpRefresh)
	if err == nil || Expired(err) || Status(err) != http.StatusInternalServerError {
		t.Fatal("a 500 is not an expired login")
	}
}
