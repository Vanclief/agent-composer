package usage

import (
	"context"
	"net/http"
	"net/http/httptest"
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
