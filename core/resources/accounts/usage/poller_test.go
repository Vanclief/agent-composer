package usage

import (
	"context"
	"sync"
	"testing"
	"time"
)

type fakeLister struct {
	mu       sync.Mutex
	accounts []Account
}

func (l *fakeLister) ListAccounts(ctx context.Context) ([]Account, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	return append([]Account{}, l.accounts...), nil
}

func (l *fakeLister) set(accounts []Account) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.accounts = accounts
}

type fakeTokens map[string]string

func (t fakeTokens) Lookup(name string) (string, bool) {
	v, ok := t[name]
	return v, ok
}

// fakeFetcher answers per token and records every call, failing the test
// when two fetches overlap.
type fakeFetcher struct {
	t        *testing.T
	mu       sync.Mutex
	inFlight int
	calls    []string
	answers  map[string]Result
}

func (f *fakeFetcher) fetch(ctx context.Context, token string) (Result, error) {
	f.mu.Lock()
	f.inFlight++
	if f.inFlight > 1 {
		f.t.Errorf("fetches overlap")
	}
	f.calls = append(f.calls, token)
	answer := f.answers[token]
	f.mu.Unlock()

	time.Sleep(time.Millisecond)

	f.mu.Lock()
	f.inFlight--
	f.mu.Unlock()

	return answer, nil
}

func (f *fakeFetcher) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()

	return len(f.calls)
}

func newTestPoller(t *testing.T, lister Lister, fetcher *fakeFetcher, now time.Time) *Poller {
	p := NewPoller(lister, "")
	p.Fetch = fetcher.fetch
	p.LoadTokens = func() (Tokens, error) {
		return fakeTokens{"ACCOUNT_WORK": "tok-work", "ACCOUNT_HOME": "tok-home", "ACCOUNT_KEY": "tok-key"}, nil
	}
	p.Now = func() time.Time {
		return now
	}

	return p
}

func TestPollerReadsAnthropicOAuthAccountsOnly(t *testing.T) {
	now := time.Date(2026, 11, 2, 16, 0, 0, 0, time.UTC)
	lister := &fakeLister{accounts: []Account{
		{Name: "work", Provider: "anthropic", Kind: "oauth", Secret: "ACCOUNT_WORK"},
		{Name: "key", Provider: "anthropic", Kind: "api_key", Secret: "ACCOUNT_KEY"},
		{Name: "home", Provider: "anthropic", Kind: "oauth", Secret: "ACCOUNT_HOME"},
		{Name: "gone", Provider: "anthropic", Kind: "oauth", Secret: "ACCOUNT_GONE"},
		{Name: "codex", Provider: "openai", Kind: "api_key", Secret: "ACCOUNT_KEY"},
	}}
	fetcher := &fakeFetcher{t: t, answers: map[string]Result{
		"tok-work": {Email: "work@example.com", Usage: UsageView{Windows: []Window{{ID: "session", Label: "Session", Used: 0.16}}}},
		"tok-home": {Usage: UsageView{Error: ErrTokenRejected}},
	}}

	var source UsageSource = newTestPoller(t, lister, fetcher, now)
	poller := source.(*Poller)
	poller.Poll(context.Background())

	if fetcher.callCount() != 2 {
		t.Fatalf("fetches = %v, want only the two oauth tokens", fetcher.calls)
	}

	work := source.Usage("work")
	if work == nil || len(work.Windows) != 1 || work.Windows[0].Used != 0.16 {
		t.Fatalf("work usage = %+v", work)
	}
	if !work.UpdatedAt.Equal(now) {
		t.Fatalf("updated_at = %v, want %v", work.UpdatedAt, now)
	}
	if source.Email("work") != "work@example.com" {
		t.Fatalf("email = %q", source.Email("work"))
	}

	home := source.Usage("home")
	if home == nil || home.Error != ErrTokenRejected || home.Windows == nil {
		t.Fatalf("home usage = %+v", home)
	}

	gone := source.Usage("gone")
	if gone == nil || gone.Error != "secret not found in secrets.env" {
		t.Fatalf("gone usage = %+v", gone)
	}

	if source.Usage("key") != nil || source.Usage("codex") != nil || source.Usage("nobody") != nil {
		t.Fatal("usage for an api_key account or an unknown one should be nil")
	}

	// A later reading replaces the first, keeps the email when the profile
	// fails, and drops accounts that are gone.
	later := now.Add(DefaultInterval)
	poller.Now = func() time.Time {
		return later
	}
	fetcher.answers["tok-work"] = Result{Usage: UsageView{Error: "profile: HTTP 502", Windows: []Window{{ID: "session", Used: 0.5}}}}
	lister.set(lister.accounts[:1])
	poller.Poll(context.Background())

	work = source.Usage("work")
	if work.Windows[0].Used != 0.5 || !work.UpdatedAt.Equal(later) || work.Error != "profile: HTTP 502" {
		t.Fatalf("work usage after the second poll = %+v", work)
	}
	if source.Email("work") != "work@example.com" {
		t.Fatalf("email lost: %q", source.Email("work"))
	}
	if source.Usage("home") != nil {
		t.Fatal("home was removed and should have no usage")
	}
}

func TestPollerStartPollsAtOnceAndEveryInterval(t *testing.T) {
	lister := &fakeLister{accounts: []Account{
		{Name: "work", Provider: "anthropic", Kind: "oauth", Secret: "ACCOUNT_WORK"},
	}}
	fetcher := &fakeFetcher{t: t, answers: map[string]Result{}}
	poller := newTestPoller(t, lister, fetcher, time.Now())
	poller.Interval = 20 * time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		poller.Start(ctx)
		close(done)
	}()

	deadline := time.Now().Add(2 * time.Second)
	for fetcher.callCount() < 3 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if fetcher.callCount() < 3 {
		t.Fatalf("fetches = %d, want a first one and one per interval", fetcher.callCount())
	}

	// Concurrent manual polls are serialized with the loop's.
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			poller.Poll(ctx)
		}()
	}
	wg.Wait()

	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Start did not return after cancel")
	}
}

func TestPollerWithNoAccountsFetchesNothing(t *testing.T) {
	fetcher := &fakeFetcher{t: t}
	poller := newTestPoller(t, nil, fetcher, time.Now())
	poller.Poll(context.Background())

	if fetcher.callCount() != 0 {
		t.Fatalf("fetches = %d", fetcher.callCount())
	}

	poller.SetLister(&fakeLister{accounts: []Account{{Name: "work", Provider: "anthropic", Kind: "oauth", Secret: "ACCOUNT_WORK"}}})
	poller.Poll(context.Background())

	if fetcher.callCount() != 1 || poller.Usage("work") == nil {
		t.Fatal("SetLister did not take effect")
	}
}
