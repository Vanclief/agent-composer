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

	channels, err := c.Channels(ctx)
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
