package client

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/sqlitedialect"
	"github.com/vanclief/ez"
	_ "modernc.org/sqlite"

	"github.com/vanclief/agent-composer/messaging"
	"github.com/vanclief/agent-composer/messaging/httpapi"
	msgmodels "github.com/vanclief/agent-composer/models/messaging"
)

func TestClientRoundTrip(t *testing.T) {
	sqldb, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	sqldb.SetMaxOpenConns(1)
	t.Cleanup(func() {
		_ = sqldb.Close()
	})

	ctx := context.Background()
	db := bun.NewDB(sqldb, sqlitedialect.New())
	tables := []interface{}{
		(*msgmodels.Participant)(nil),
		(*msgmodels.Message)(nil),
		(*msgmodels.ChannelArchive)(nil),
	}
	for _, table := range tables {
		_, err = db.NewCreateTable().Model(table).Exec(ctx)
		if err != nil {
			t.Fatal(err)
		}
	}

	task := func(*http.Request) (messaging.Caller, error) {
		return messaging.Caller{Task: "review-42"}, nil
	}
	server := httptest.NewServer(httpapi.New(messaging.NewService(db), task))
	t.Cleanup(server.Close)

	c := &Client{BaseURL: server.URL + "/"}

	p, err := c.Join(ctx, "reviewer")
	if err != nil {
		t.Fatal(err)
	}

	m, err := c.Post(ctx, p.ID, "#review-42", "hi", nil)
	if err != nil {
		t.Fatal(err)
	}

	reply, err := c.Post(ctx, p.ID, "#review-42", "again", &m.ID)
	if err != nil {
		t.Fatal(err)
	}

	read, err := c.Read(ctx, "#review-42", m.ID, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(read.Messages) != 1 || read.Cursor != reply.ID || *read.Messages[0].ReplyTo != m.ID {
		t.Fatalf("unexpected read %+v", read)
	}

	waited, err := c.Wait(ctx, "#review-42", reply.ID, 10*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	if len(waited.Messages) != 0 {
		t.Fatalf("want empty wait, got %+v", waited)
	}

	channels, err := c.Channels(ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(channels.Channels) != 1 || channels.Channels[0].LastID != reply.ID {
		t.Fatalf("unexpected channels %+v", channels)
	}

	_, err = c.Post(ctx, p.ID, "#releases", "hi", nil)
	if ez.ErrorCode(err) != ez.ENOTAUTHORIZED || ez.ErrorMessage(err) != "channel #releases is not available to task review-42" {
		t.Fatalf("want the server's 403 as an ez error, got %v", err)
	}

	left, err := c.Leave(ctx, p.ID)
	if err != nil || !left.Left {
		t.Fatalf("leave: %+v %v", left, err)
	}
}

// The home sandbox's agent routes are served by agents-core; this fakes
// their contract (ADR 0016): GET /agent and POST /agent/sleep → 204.
func TestClientAgentAndSleep(t *testing.T) {
	slept := false

	mux := http.NewServeMux()
	mux.HandleFunc("GET "+httpapi.AgentPath, func(w http.ResponseWriter, r *http.Request) {
		httpapi.WriteJSON(w, http.StatusOK, map[string]any{
			"agent":    map[string]any{"name": "reviewer-1", "role": "reviewer", "owner": "franco", "kept": false},
			"channels": []string{"#reviewer-1", "#fix-login"},
			"idle":     "15m",
		})
	})
	mux.HandleFunc("POST "+httpapi.AgentSleepPath, func(w http.ResponseWriter, r *http.Request) {
		slept = true
		w.WriteHeader(http.StatusNoContent)
	})

	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	c := &Client{BaseURL: server.URL}

	res, err := c.Agent(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if res.Agent.Name != "reviewer-1" || res.Agent.Role != "reviewer" || len(res.Channels) != 2 || res.Idle != "15m" {
		t.Fatalf("agent = %+v", res)
	}

	err = c.Sleep(context.Background())
	if err != nil || !slept {
		t.Fatalf("sleep: %v, slept %v", err, slept)
	}
}
