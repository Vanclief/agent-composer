package usage

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/vanclief/ez"
)

// fakeLogins hands out one access token, or an error.
type fakeLogins struct {
	token string
	err   error
}

func (f fakeLogins) AccessToken(ctx context.Context, name string) (string, error) {
	return f.token, f.err
}

// An account with no login whose profile answers 403 reads as
// inference-only; one with a login keeps the plain why.
func TestPollerMapsInferenceOnly(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == ProfilePath {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		_, _ = w.Write([]byte(`{"five_hour":{"utilization":10}}`))
	}))
	t.Cleanup(srv.Close)

	lister := &fakeLister{}
	lister.set([]Account{{Name: "work", Provider: "anthropic", Kind: "oauth", Secret: "ACCOUNT_WORK"}})

	p := NewPoller(lister, "")
	fetcher := Fetcher{BaseURL: srv.URL}
	p.Fetch = func(ctx context.Context, token string) (Result, error) {
		return fetcher.Fetch(ctx, srv.Client(), token)
	}
	p.LoadTokens = func() (Tokens, error) {
		return fakeTokens{"ACCOUNT_WORK": "made-up-inference"}, nil
	}

	p.Poll(context.Background())
	if p.Usage("work").Error != ErrInferenceOnly {
		t.Fatalf("no login: %q", p.Usage("work").Error)
	}

	lister.set([]Account{{Name: "work", Provider: "anthropic", Kind: "oauth", Secret: "ACCOUNT_WORK", Login: true}})
	p.SetLogins(fakeLogins{token: "made-up-access"})

	p.Poll(context.Background())
	if p.Usage("work").Error != "profile: HTTP 403" {
		t.Fatalf("with a login: %q", p.Usage("work").Error)
	}

	// The login is gone: sign-in expired, the last windows kept.
	p.SetLogins(fakeLogins{err: ez.New(ez.ENOTFOUND, "account work is not signed in", nil)})
	p.Now = func() time.Time {
		return time.Now().Add(time.Hour)
	}

	p.Poll(context.Background())
	reading := p.Usage("work")
	if reading.Error != ErrSignInExpired || len(reading.Windows) != 1 {
		t.Fatalf("login gone: %+v", reading)
	}
	if p.PollAccount(context.Background(), "work") != "" {
		t.Fatal("an email from nowhere")
	}
}

// For an account with no login the profile status decides: 403 is
// inference-only whatever usage said, 401 a rejected token, 5xx the
// status text. The token never shows in the reading.
func TestPollerTellsInferenceOnlyFromRejected(t *testing.T) {
	cases := []struct {
		name    string
		usage   func(w http.ResponseWriter)
		profile func(w http.ResponseWriter)
		want    string
		windows int
	}{
		{"usage 401, profile 403", status(http.StatusUnauthorized), status(http.StatusForbidden), ErrInferenceOnly, 0},
		{"usage 401, profile 401", status(http.StatusUnauthorized), status(http.StatusUnauthorized), ErrTokenRejected, 0},
		{"usage 200, profile 403", body(usageResponse), status(http.StatusForbidden), ErrInferenceOnly, 4},
		{"usage 429, profile 403", status(http.StatusTooManyRequests), status(http.StatusForbidden), ErrInferenceOnly, 0},
		{"usage 429, profile 503", status(http.StatusTooManyRequests), status(http.StatusServiceUnavailable), "usage: HTTP 429, profile: HTTP 503", 0},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			srv, _ := newServer(t, c.usage, c.profile)

			lister := &fakeLister{}
			lister.set([]Account{{Name: "work", Provider: "anthropic", Kind: "oauth", Secret: "ACCOUNT_WORK"}})

			p := NewPoller(lister, "")
			fetcher := Fetcher{BaseURL: srv.URL}
			p.Fetch = func(ctx context.Context, token string) (Result, error) {
				return fetcher.Fetch(ctx, srv.Client(), token)
			}
			p.LoadTokens = func() (Tokens, error) {
				return fakeTokens{"ACCOUNT_WORK": fakeToken}, nil
			}

			p.Poll(context.Background())
			reading := p.Usage("work")
			if reading.Error != c.want {
				t.Fatalf("error %q, want %q", reading.Error, c.want)
			}
			if len(reading.Windows) != c.windows {
				t.Fatalf("windows %+v, want %d", reading.Windows, c.windows)
			}
			if strings.Contains(reading.Error, fakeToken) {
				t.Fatal("token leaked into the error")
			}
			assertNoToken(t, Result{Usage: *reading, Email: p.Email("work")}, nil)
		})
	}
}
