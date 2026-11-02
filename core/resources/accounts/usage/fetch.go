// Package usage reads an Anthropic subscription account's identity and
// usage bars from the Anthropic OAuth API, and keeps the latest reading per
// account in memory for GET /api/accounts (the accounts contract: AccountView
// carries `usage: UsageView | null`).
//
// # What Claude Code calls
//
// Read from the Claude Code bundle, version 2.1.316 (BUILD_TIME
// 2026-10-30T23:28:13Z, /usr/local/lib/node_modules/@anthropic-ai/claude-code/bin/claude.exe),
// with `strings` around "api/oauth/usage" and "api/oauth/profile". Both are
// GET requests on BASE_API_URL, https://api.anthropic.com in production,
// with the subscription's OAuth access token as a Bearer token.
//
// GET /api/oauth/profile
//
//	Authorization: Bearer <token>
//	Content-Type: application/json
//	Cache-Control: no-cache
//	User-Agent: claude-code/<version>
//
// The bundle validates the body as
// {account: {uuid, email}, organization: {uuid}} (extra fields pass
// through); the email is account.email.
//
// GET /api/oauth/usage (the /usage panel's fetchUtilization; the bundle also
// knows the query forms ?at_wall=1&skip_spend=1 and ?cedar_ember=1&skip_spend=1,
// the plain path is the default read)
//
//	Authorization: Bearer <token>
//	anthropic-beta: oauth-2025-04-20   (sent with every OAuth bearer request)
//	Content-Type: application/json
//
// The body (the bundle's schema, every field nullish, extra fields pass
// through):
//
//	five_hour, seven_day, seven_day_oauth_apps, seven_day_opus, seven_day_sonnet:
//	    {utilization: number, resets_at: string ISO 8601} or null
//	extra_usage: {is_enabled, monthly_limit, used_credits, utilization, currency}
//	limits: [{kind, group, percent, resets_at, severity, is_active,
//	          scope: {model: {display_name}, surface: {display_name}}}]
//
// utilization and percent are whole percents, 0 to 100 (above 100 once
// over). A per-model weekly window is a limits row with
// kind "weekly_scoped" (group "weekly") and a scope.model.display_name,
// e.g. "Fable".
//
// # Mapping
//
// five_hour → window "session" ("Session"), seven_day → "weekly"
// ("Weekly"), seven_day_opus and seven_day_sonnet → "opus" ("Opus") and
// "sonnet" ("Sonnet"), each weekly_scoped limits row → the slug of its model
// name with the name as label (a limits row wins over a seven_day_<model>
// field of the same slug). used is the percent as a fraction clamped to
// 0..1; resets_at is RFC 3339 UTC, "" when the API gives none.
//
// An endpoint that fails or changes shape yields UsageView.Error with a
// short why and the windows that could be read. The token is never put in
// an error, a log line or a result.
package usage

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/vanclief/ez"
)

const (
	// DefaultBaseURL is the Anthropic OAuth API host.
	DefaultBaseURL = "https://api.anthropic.com"
	// UsagePath and ProfilePath are the two endpoints Claude Code reads.
	UsagePath   = "/api/oauth/usage"
	ProfilePath = "/api/oauth/profile"
	// OAuthBeta is the anthropic-beta value Claude Code sends with OAuth
	// bearer requests.
	OAuthBeta = "oauth-2025-04-20"
	// DefaultUserAgent identifies agc to the API.
	DefaultUserAgent = "agc"
	// ErrTokenRejected is UsageView.Error when the API answers 401.
	ErrTokenRejected = "token rejected: sign in again"
	// ErrInferenceOnly is UsageView.Error for an account with no login
	// whose token the API refuses for usage or profile: a token from
	// `claude setup-token` is inference-only (ADR 0018 "Sign-in").
	ErrInferenceOnly = "inference-only token: sign in for usage"
	// ErrSignInExpired is UsageView.Error when the login's refresh token
	// is refused or gone.
	ErrSignInExpired = "sign-in expired: sign in again"

	maxBody        = 1 << 20
	requestTimeout = 10 * time.Second
)

// Window is one usage bar.
type Window struct {
	// ID is session, weekly or a model slug.
	ID    string `json:"id"`
	Label string `json:"label"`
	// Used is the fraction of the window used, 0..1.
	Used float64 `json:"used"`
	// ResetsAt is RFC 3339 UTC, "" when unknown.
	ResetsAt string `json:"resets_at"`
}

// UsageView is an account's latest usage reading.
type UsageView struct {
	UpdatedAt time.Time `json:"updated_at"`
	Windows   []Window  `json:"windows"`
	// Error is "" or a short why the reading is incomplete.
	Error string `json:"error"`
}

// Result is one reading of an account: its email and its usage.
type Result struct {
	Email string
	Usage UsageView

	// usageStatus and profileStatus are each endpoint's HTTP status, 0
	// when it was not reached; the poller maps them for an account with
	// no login.
	usageStatus   int
	profileStatus int
}

// Fetcher reads the usage and profile endpoints. The zero value uses
// DefaultBaseURL and DefaultUserAgent.
type Fetcher struct {
	BaseURL   string
	UserAgent string
}

// Fetch reads token's usage and profile from the Anthropic OAuth API with
// client (http.DefaultClient when nil).
func Fetch(ctx context.Context, client *http.Client, token string) (Result, error) {
	return Fetcher{}.Fetch(ctx, client, token)
}

// Fetch reads token's usage and profile. Endpoint failures go in
// Result.Usage.Error, never in the returned error, which is only for an
// empty token or a cancelled context.
func (f Fetcher) Fetch(ctx context.Context, client *http.Client, token string) (Result, error) {
	if token == "" {
		return Result{}, ez.New(ez.EINVALID, "token is required", nil)
	}

	if client == nil {
		client = http.DefaultClient
	}

	var whys []string
	result := Result{}

	var body usageBody
	why, status := f.get(ctx, client, token, UsagePath, &body)
	result.usageStatus = status
	if why != "" {
		whys = append(whys, why)
	}
	result.Usage.Windows = body.windows()

	var profile profileBody
	why, status = f.get(ctx, client, token, ProfilePath, &profile)
	result.profileStatus = status
	if why != "" {
		whys = append(whys, why)
	}
	if profile.Account != nil {
		result.Email = profile.Account.Email
	}

	err := ctx.Err()
	if err != nil {
		return Result{}, ez.Wrap(err)
	}

	result.Usage.Error = joinWhys(whys)
	result.Usage.UpdatedAt = time.Now().UTC()

	return result, nil
}

// get GETs path into out and returns "" or a short why, and the HTTP
// status (0 when there was no answer). The why never carries the token or
// the transport's error text.
func (f Fetcher) get(ctx context.Context, client *http.Client, token, path string, out interface{}) (string, int) {
	name := strings.TrimPrefix(path, "/api/oauth/")

	base := f.BaseURL
	if base == "" {
		base = DefaultBaseURL
	}

	userAgent := f.UserAgent
	if userAgent == "" {
		userAgent = DefaultUserAgent
	}

	reqCtx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, strings.TrimSuffix(base, "/")+path, nil)
	if err != nil {
		return name + ": bad base URL", 0
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("anthropic-beta", OAuthBeta)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("User-Agent", userAgent)

	resp, err := client.Do(req)
	if err != nil {
		if reqCtx.Err() == context.DeadlineExceeded && ctx.Err() == nil {
			return name + ": timed out", 0
		}
		return name + ": unreachable", 0
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return ErrTokenRejected, resp.StatusCode
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Sprintf("%s: HTTP %d", name, resp.StatusCode), resp.StatusCode
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	if err != nil {
		return name + ": reading the response failed", resp.StatusCode
	}

	err = json.Unmarshal(data, out)
	if err != nil {
		return name + ": unexpected response", resp.StatusCode
	}

	return "", resp.StatusCode
}

// joinWhys joins distinct whys, so a 401 on both endpoints reads once.
func joinWhys(whys []string) string {
	var seen []string
	for _, why := range whys {
		dup := false
		for _, s := range seen {
			if s == why {
				dup = true
			}
		}
		if !dup {
			seen = append(seen, why)
		}
	}

	return strings.Join(seen, ", ")
}

type profileBody struct {
	Account *struct {
		Email string `json:"email"`
	} `json:"account"`
}

type windowBody struct {
	Utilization *float64 `json:"utilization"`
	ResetsAt    *string  `json:"resets_at"`
}

type limitBody struct {
	Kind     string   `json:"kind"`
	Group    string   `json:"group"`
	Percent  *float64 `json:"percent"`
	ResetsAt *string  `json:"resets_at"`
	Scope    *struct {
		Model *struct {
			DisplayName string `json:"display_name"`
		} `json:"model"`
	} `json:"scope"`
}

type usageBody struct {
	FiveHour       *windowBody `json:"five_hour"`
	SevenDay       *windowBody `json:"seven_day"`
	SevenDayOpus   *windowBody `json:"seven_day_opus"`
	SevenDaySonnet *windowBody `json:"seven_day_sonnet"`
	Limits         []limitBody `json:"limits"`
}

func (b usageBody) windows() []Window {
	windows := []Window{}

	add := func(id, label string, percent *float64, resetsAt *string) {
		if percent == nil {
			return
		}
		w := Window{ID: id, Label: label, Used: fraction(*percent), ResetsAt: rfc3339(resetsAt)}
		for i := range windows {
			if windows[i].ID == id {
				windows[i] = w
				return
			}
		}
		windows = append(windows, w)
	}

	fixed := []struct {
		id, label string
		w         *windowBody
	}{
		{"session", "Session", b.FiveHour},
		{"weekly", "Weekly", b.SevenDay},
		{"opus", "Opus", b.SevenDayOpus},
		{"sonnet", "Sonnet", b.SevenDaySonnet},
	}
	for _, f := range fixed {
		if f.w != nil {
			add(f.id, f.label, f.w.Utilization, f.w.ResetsAt)
		}
	}

	for _, l := range b.Limits {
		if l.Kind != "weekly_scoped" || l.Scope == nil || l.Scope.Model == nil {
			continue
		}
		name := strings.TrimSpace(l.Scope.Model.DisplayName)
		id := slug(name)
		if id == "" || id == "session" || id == "weekly" {
			continue
		}
		add(id, name, l.Percent, l.ResetsAt)
	}

	return windows
}

// fraction turns a whole percent into 0..1.
func fraction(percent float64) float64 {
	if math.IsNaN(percent) {
		return 0
	}

	return math.Min(1, math.Max(0, percent/100))
}

// rfc3339 normalizes an ISO 8601 time to RFC 3339 UTC, "" when absent or
// unreadable.
func rfc3339(s *string) string {
	if s == nil || *s == "" {
		return ""
	}

	t, err := time.Parse(time.RFC3339Nano, *s)
	if err != nil {
		return ""
	}

	return t.UTC().Format(time.RFC3339)
}

var nonSlug = regexp.MustCompile(`[^a-z0-9]+`)

// slug turns a model name into a window id: "Claude Opus 4" → "claude-opus-4".
func slug(name string) string {
	return strings.Trim(nonSlug.ReplaceAllString(strings.ToLower(name), "-"), "-")
}
