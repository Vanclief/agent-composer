package migrations

import (
	"context"
	"testing"
	"time"

	"github.com/vanclief/agent-composer/models/messaging"
)

func TestMessageImagesAddsColumn(t *testing.T) {
	ctx := context.Background()
	db := newSQLite(t)

	// messaging_messages as it was before images.
	_, err := db.ExecContext(ctx, `CREATE TABLE messaging_messages (id INTEGER PRIMARY KEY AUTOINCREMENT, channel TEXT NOT NULL,
"from" TEXT NOT NULL, participant_id TEXT NOT NULL, task TEXT NOT NULL, agent TEXT NOT NULL DEFAULT '',
human BOOLEAN NOT NULL DEFAULT false, reply_to INTEGER, body TEXT NOT NULL, kind TEXT NOT NULL DEFAULT 'message',
created_at TIMESTAMP NOT NULL)`)
	if err != nil {
		t.Fatal(err)
	}

	now := time.Date(2026, 11, 3, 2, 0, 0, 0, time.UTC)
	_, err = db.ExecContext(ctx, `INSERT INTO messaging_messages (channel, "from", participant_id, task, body, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
		"#talk", "franco", "p1", "", "text only", now)
	if err != nil {
		t.Fatal(err)
	}

	// Twice: the second run, as on a fresh database, changes nothing.
	for range 2 {
		err = messageImages(ctx, db)
		if err != nil {
			t.Fatal(err)
		}
	}

	// The later column too, so the current model can write messages.
	err = messageTurnFor(ctx, db)
	if err != nil {
		t.Fatal(err)
	}

	m := &messaging.Message{Channel: "#talk", From: "franco", ParticipantID: "p1",
		Images: []messaging.Image{{Name: "shot.png", Type: "image/png", Size: 12}}}
	err = m.Insert(ctx, db)
	if err != nil {
		t.Fatal(err)
	}

	messages := []messaging.Message{}
	err = db.NewSelect().Model(&messages).Order("id ASC").Scan(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if len(messages) != 2 || len(messages[0].Images) != 0 || messages[0].Body != "text only" {
		t.Fatalf("messages = %+v", messages)
	}

	got := messages[1].Images
	if len(got) != 1 || got[0].Name != "shot.png" || got[0].Size != 12 || got[0].URL != "/api/msg/images/2/shot.png" {
		t.Fatalf("images = %+v", got)
	}
}
