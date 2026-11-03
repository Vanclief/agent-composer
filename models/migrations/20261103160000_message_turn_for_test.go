package migrations

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/vanclief/agent-composer/models/messaging"
)

func TestMessageTurnForAddsColumn(t *testing.T) {
	ctx := context.Background()
	db := newSQLite(t)

	// messaging_messages as it was before turn_for.
	_, err := db.ExecContext(ctx, `CREATE TABLE messaging_messages (id INTEGER PRIMARY KEY AUTOINCREMENT, channel TEXT NOT NULL,
"from" TEXT NOT NULL, participant_id TEXT NOT NULL, task TEXT NOT NULL, agent TEXT NOT NULL DEFAULT '',
human BOOLEAN NOT NULL DEFAULT false, reply_to INTEGER, body TEXT NOT NULL, kind TEXT NOT NULL DEFAULT 'message',
images TEXT, created_at TIMESTAMP NOT NULL)`)
	if err != nil {
		t.Fatal(err)
	}

	now := time.Date(2026, 11, 3, 16, 0, 0, 0, time.UTC)
	_, err = db.ExecContext(ctx, `INSERT INTO messaging_messages (channel, "from", participant_id, task, body, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
		"#designer-1", "franco", "p1", "", "before", now)
	if err != nil {
		t.Fatal(err)
	}

	// Twice: the second run, as on a fresh database, changes nothing.
	for range 2 {
		err = messageTurnFor(ctx, db)
		if err != nil {
			t.Fatal(err)
		}
	}

	m := &messaging.Message{Channel: "#designer-1", From: "franco", ParticipantID: "p1", Body: "after", TurnFor: []string{"designer-1"}}
	err = m.Insert(ctx, db)
	if err != nil {
		t.Fatal(err)
	}

	messages := []messaging.Message{}
	err = db.NewSelect().Model(&messages).Order("id ASC").Scan(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if len(messages) != 2 || len(messages[0].TurnFor) != 0 || messages[0].Body != "before" {
		t.Fatalf("messages = %+v", messages)
	}

	if !reflect.DeepEqual(messages[1].TurnFor, []string{"designer-1"}) {
		t.Fatalf("turn_for = %q", messages[1].TurnFor)
	}
}
