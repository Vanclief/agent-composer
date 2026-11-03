package core

import (
	"bytes"
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/sqlitedialect"
	_ "modernc.org/sqlite"

	"github.com/vanclief/agent-composer/core/resources/accounts"
	"github.com/vanclief/agent-composer/core/resources/accounts/usage"
	"github.com/vanclief/agent-composer/models/account"
)

// A made-up value: no test holds a real credential or calls the real
// endpoint.
const madeUpToken = "made-up-oauth-value"

// The poller and the accounts API connected as NewStack connects them:
// after one poll GET /api/accounts shows the fake fetcher's reading.
func TestAccountsUsageWiring(t *testing.T) {
	ctx := context.Background()

	sqldb, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	sqldb.SetMaxOpenConns(1)
	t.Cleanup(func() {
		_ = sqldb.Close()
	})

	db := bun.NewDB(sqldb, sqlitedialect.New())
	_, err = db.NewCreateTable().Model((*account.Account)(nil)).Exec(ctx)
	if err != nil {
		t.Fatal(err)
	}

	logged := &bytes.Buffer{}
	previous := log.Logger
	log.Logger = zerolog.New(logged)
	t.Cleanup(func() {
		log.Logger = previous
	})

	secretsPath := filepath.Join(t.TempDir(), "secrets.env")
	api := accounts.New(db, secretsPath, nil)
	poller := usage.NewPoller(usage.NoAccounts{}, secretsPath)

	updated := time.Date(2026, 11, 2, 16, 50, 0, 0, time.UTC)
	resets := time.Date(2026, 11, 2, 21, 0, 0, 0, time.UTC)
	fetched := []string{}
	poller.Fetch = func(ctx context.Context, token string) (usage.Result, error) {
		fetched = append(fetched, token)
		return usage.Result{
			Email: "someone@example.com",
			Usage: usage.UsageView{
				UpdatedAt: updated,
				Windows: []usage.Window{
					{ID: "session", Label: "Session", Used: 0.25, ResetsAt: resets.Format(time.RFC3339)},
					{ID: "weekly", Label: "Weekly", Used: 0.5},
				},
			},
		}, nil
	}

	connectAccountsUsage(api, poller)

	_, err = api.Create(ctx, nil, &accounts.CreateRequest{Name: "work", Provider: account.ProviderAnthropic, Kind: account.KindOAuth, Value: madeUpToken})
	if err != nil {
		t.Fatal(err)
	}

	list, err := api.List(ctx, nil, &accounts.ListRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if len(list.Accounts) != 1 || list.Accounts[0].Usage != nil {
		t.Fatalf("before the first poll: %+v", list.Accounts)
	}

	poller.Poll(ctx)

	if len(fetched) != 1 || fetched[0] != madeUpToken {
		t.Fatalf("fetched %d tokens, want the account's one", len(fetched))
	}

	list, err = api.List(ctx, nil, &accounts.ListRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if len(list.Accounts) != 1 {
		t.Fatalf("accounts = %+v", list.Accounts)
	}

	got := list.Accounts[0].Usage
	if got == nil {
		t.Fatal("usage is null after the first poll")
	}
	if !got.UpdatedAt.Equal(updated) || got.Error != "" || len(got.Windows) != 2 {
		t.Fatalf("usage = %+v", got)
	}

	session := got.Windows[0]
	if session.ID != "session" || session.Label != "Session" || session.Used != 0.25 || session.ResetsAt == nil || !session.ResetsAt.Equal(resets) {
		t.Fatalf("session window = %+v", session)
	}

	weekly := got.Windows[1]
	if weekly.ID != "weekly" || weekly.Used != 0.5 || weekly.ResetsAt != nil {
		t.Fatalf("weekly window = %+v", weekly)
	}

	// One info line per cycle, counts only.
	lines := strings.Split(strings.TrimSpace(logged.String()), "\n")
	polled := []string{}
	for _, line := range lines {
		if strings.Contains(line, `"message":"accounts usage: polled"`) {
			polled = append(polled, line)
		}
	}
	if len(polled) != 1 {
		t.Fatalf("poll log lines = %q", polled)
	}
	if !strings.Contains(polled[0], `"level":"info"`) || !strings.Contains(polled[0], `"accounts":1`) || !strings.Contains(polled[0], `"errors":0`) {
		t.Fatalf("poll log line = %s", polled[0])
	}
	if strings.Contains(logged.String(), madeUpToken) || strings.Contains(logged.String(), "someone@example.com") {
		t.Fatal("the log carries the token or the email")
	}
}
