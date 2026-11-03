package migrations

import (
	"context"
	"testing"
	"time"

	msgmodels "github.com/vanclief/agent-composer/models/messaging"
)

func TestMessageKindMakesExistingRowsMessages(t *testing.T) {
	ctx := context.Background()
	db := newSQLite(t)

	// The messages table as ADR 0009 left it.
	_, err := db.ExecContext(ctx, `CREATE TABLE messaging_messages (id INTEGER PRIMARY KEY AUTOINCREMENT, channel TEXT NOT NULL,
"from" TEXT NOT NULL, participant_id TEXT NOT NULL, task TEXT NOT NULL, human BOOLEAN NOT NULL DEFAULT false,
reply_to INTEGER, body TEXT NOT NULL, created_at TIMESTAMP NOT NULL)`)
	if err != nil {
		t.Fatal(err)
	}

	now := time.Date(2026, 11, 2, 9, 30, 0, 0, time.UTC)
	_, err = db.ExecContext(ctx, `INSERT INTO messaging_messages (channel, "from", participant_id, task, body, created_at)
VALUES (?, ?, ?, ?, ?, ?)`, "#stop-icon", "franco", "p1", "", "hello", now)
	if err != nil {
		t.Fatal(err)
	}

	// Twice: the second run, as on a fresh database, changes nothing.
	for range 2 {
		err = messageKind(ctx, db)
		if err != nil {
			t.Fatal(err)
		}
	}

	// The later columns too (createAgents, messageImages), so the current model can read
	// the rows.
	err = addColumn(ctx, db, "messaging_messages", "agent", "TEXT NOT NULL DEFAULT ''")
	if err != nil {
		t.Fatal(err)
	}
	err = messageImages(ctx, db)
	if err != nil {
		t.Fatal(err)
	}

	var messages []msgmodels.Message
	err = db.NewSelect().Model(&messages).Scan(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(messages) != 1 || messages[0].Kind != msgmodels.KindMessage {
		t.Fatalf("messages = %+v", messages)
	}

	question := &msgmodels.Message{Channel: "#stop-icon", From: "agent", ParticipantID: "p2", Body: "which icon?", Kind: msgmodels.KindQuestion}
	err = question.Insert(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
}
