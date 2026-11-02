// Package oauth runs the OAuth login Claude Code runs (`claude auth login`,
// "Claude account with subscription", manual redirect), done by agc per
// account so the usage poller reads usage and profile with a full-scope
// token (ADR 0018 "Sign-in"). It imports nothing of accounts, so usage can
// import it. No error it returns holds a token or a code.
//
// # What Claude Code does
//
// Read from the Claude Code bundle, version 2.1.316 (/usr/local/bin/claude),
// with `strings` around MANUAL_REDIRECT_URL. The production config (the one
// whose TOKEN_URL is https://platform.claude.com/v1/oauth/token; the other,
// with client id 22422756-…, is the "-local-oauth" config pointing at
// localhost) holds:
//
//	CLAUDE_AI_AUTHORIZE_URL: "https://claude.com/cai/oauth/authorize"
//	CONSOLE_AUTHORIZE_URL:   "https://platform.claude.com/oauth/authorize"
//	TOKEN_URL:               "https://platform.claude.com/v1/oauth/token"
//	MANUAL_REDIRECT_URL:     "https://platform.claude.com/oauth/code/callback"
//	CLIENT_ID:               "9d1c250a-e61b-44d9-88ed-5944d1962f5e"
//
// The authorize URL is CLAUDE_AI_AUTHORIZE_URL when the login is with a
// claude.ai subscription (loginWithClaudeAi), CONSOLE_AUTHORIZE_URL
// otherwise; accounts are subscriptions, so agc uses the claude.ai one. Its
// query: code=true, client_id, response_type=code, redirect_uri (the manual
// one: the page then shows `code#state` to paste), scope (space separated),
// code_challenge, code_challenge_method=S256, state.
//
// The exchange POSTs JSON to TOKEN_URL:
// {grant_type: "authorization_code", code, redirect_uri, client_id,
// code_verifier, state}; a refresh POSTs
// {grant_type: "refresh_token", refresh_token, client_id, scope}. Both
// answer {access_token, refresh_token, expires_in, scope}; a refresh with
// no refresh_token keeps the old one. Scopes: "user:inference" and
// "user:profile" (the bundle's oT and pV); /api/oauth/usage and
// /api/oauth/profile need user:profile.
package oauth

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/vanclief/ez"
)

// The production values, Claude Code 2.1.316 (see the package comment).
const (
	DefaultAuthorizeURL = "https://claude.com/cai/oauth/authorize"
	DefaultTokenURL     = "https://platform.claude.com/v1/oauth/token"
	DefaultRedirectURL  = "https://platform.claude.com/oauth/code/callback"
	DefaultClientID     = "9d1c250a-e61b-44d9-88ed-5944d1962f5e"
)

// DefaultScopes are the scopes agc asks for: user:profile for usage and
// identity, user:inference as Claude Code's login has it.
var DefaultScopes = []string{"user:profile", "user:inference"}

// PendingFor is how long a begun sign-in waits for its code.
const PendingFor = 10 * time.Minute

const (
	maxBody        = 1 << 20
	requestTimeout = 30 * time.Second
	// statusKey is the ez.Error data key holding the token endpoint's
	// HTTP status.
	statusKey = "status"
)

// Config is where and as whom agc signs in.
type Config struct {
	AuthorizeURL string
	TokenURL     string
	RedirectURL  string
	ClientID     string
	Scopes       []string
}

// Default is the production config.
func Default() Config {
	return Config{
		AuthorizeURL: DefaultAuthorizeURL,
		TokenURL:     DefaultTokenURL,
		RedirectURL:  DefaultRedirectURL,
		ClientID:     DefaultClientID,
		Scopes:       append([]string{}, DefaultScopes...),
	}
}

// Pending is a begun sign-in: its state, its PKCE verifier and challenge,
// and when it lapses.
type Pending struct {
	State     string
	Verifier  string
	Challenge string
	ExpiresAt time.Time
}

// Token is what the token endpoint gave.
type Token struct {
	Access    string
	Refresh   string
	ExpiresAt time.Time
	Scope     string
}

// Begin starts a sign-in: a random state, a PKCE verifier (43 characters)
// and its S256 challenge, and the authorize URL to open.
func (c Config) Begin() (Pending, string) {
	verifier := randomString(32)
	sum := sha256.Sum256([]byte(verifier))

	pending := Pending{
		State:     randomString(32),
		Verifier:  verifier,
		Challenge: base64.RawURLEncoding.EncodeToString(sum[:]),
		ExpiresAt: time.Now().Add(PendingFor),
	}

	query := url.Values{}
	query.Set("code", "true")
	query.Set("client_id", c.ClientID)
	query.Set("response_type", "code")
	query.Set("redirect_uri", c.RedirectURL)
	query.Set("scope", strings.Join(c.Scopes, " "))
	query.Set("code_challenge", pending.Challenge)
	query.Set("code_challenge_method", "S256")
	query.Set("state", pending.State)

	return pending, c.AuthorizeURL + "?" + query.Encode()
}

// randomString is n random bytes, url-safe base64 without padding.
func randomString(n int) string {
	buf := make([]byte, n)
	// crypto/rand.Read never returns an error since Go 1.24: it crashes
	// the program instead.
	_, _ = rand.Read(buf)

	return base64.RawURLEncoding.EncodeToString(buf)
}

// Exchange trades the pasted code for tokens.
func Exchange(ctx context.Context, client *http.Client, cfg Config, code, state, verifier string) (Token, error) {
	body := map[string]string{
		"grant_type":    "authorization_code",
		"code":          code,
		"state":         state,
		"client_id":     cfg.ClientID,
		"redirect_uri":  cfg.RedirectURL,
		"code_verifier": verifier,
	}

	token, err := post(ctx, client, cfg, body)
	if err != nil {
		return Token{}, ez.Wrap(err)
	}

	if token.Refresh == "" {
		return Token{}, ez.New(ez.EINVALID, "sign-in failed: the token endpoint gave no refresh token", nil)
	}

	return token, nil
}

// Refresh trades a refresh token for a new access token. The returned
// Refresh is the rotated refresh token, or refresh when none came back.
func Refresh(ctx context.Context, client *http.Client, cfg Config, refresh string) (Token, error) {
	body := map[string]string{
		"grant_type":    "refresh_token",
		"refresh_token": refresh,
		"client_id":     cfg.ClientID,
		"scope":         strings.Join(cfg.Scopes, " "),
	}

	token, err := post(ctx, client, cfg, body)
	if err != nil {
		return Token{}, ez.Wrap(err)
	}

	if token.Refresh == "" {
		token.Refresh = refresh
	}

	return token, nil
}

type tokenBody struct {
	AccessToken  string  `json:"access_token"`
	RefreshToken string  `json:"refresh_token"`
	ExpiresIn    float64 `json:"expires_in"`
	Scope        string  `json:"scope"`
}

// post sends body as JSON to the token URL. Its errors carry the HTTP
// status (Status reads it) and never the request or response body, which
// hold the code and the tokens.
func post(ctx context.Context, client *http.Client, cfg Config, body map[string]string) (Token, error) {
	if client == nil {
		client = http.DefaultClient
	}

	data, err := json.Marshal(body)
	if err != nil {
		return Token{}, ez.Wrap(err)
	}

	reqCtx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, cfg.TokenURL, bytes.NewReader(data))
	if err != nil {
		return Token{}, ez.New(ez.EINTERNAL, "sign-in: bad token URL", nil)
	}
	req.Header.Set("Content-Type", "application/json")

	start := time.Now()

	resp, err := client.Do(req)
	if err != nil {
		// The transport's error text could hold the URL, never a token,
		// but it is left out all the same.
		return Token{}, ez.New(ez.EUNAVAILABLE, "token endpoint unreachable", nil)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		statusErr := ez.New(ez.EINVALID, fmt.Sprintf("token endpoint answered HTTP %d", resp.StatusCode), nil)
		statusErr.Data = map[string]interface{}{statusKey: resp.StatusCode}
		return Token{}, statusErr
	}

	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	if err != nil {
		return Token{}, ez.New(ez.EUNAVAILABLE, "token endpoint: reading the response failed", nil)
	}

	var parsed tokenBody
	err = json.Unmarshal(raw, &parsed)
	if err != nil || parsed.AccessToken == "" {
		return Token{}, ez.New(ez.EINTERNAL, "token endpoint: unexpected response", nil)
	}

	return Token{
		Access:    parsed.AccessToken,
		Refresh:   parsed.RefreshToken,
		ExpiresAt: start.Add(time.Duration(parsed.ExpiresIn * float64(time.Second))),
		Scope:     parsed.Scope,
	}, nil
}

// Status is the token endpoint's HTTP status behind err, 0 when err did
// not come from an answer that was not 200.
func Status(err error) int {
	for err != nil {
		e, ok := err.(*ez.Error)
		if !ok {
			return 0
		}

		status, ok := e.Data[statusKey].(int)
		if ok {
			return status
		}

		err = e.Err
	}

	return 0
}

// Expired reports whether err is the token endpoint refusing the refresh
// token (400 or 401): the login is gone and the user must sign in again.
func Expired(err error) bool {
	status := Status(err)

	return status == http.StatusBadRequest || status == http.StatusUnauthorized
}
