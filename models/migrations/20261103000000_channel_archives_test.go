package migrations

import (
	"context"
	"testing"
	"time"

	"github.com/vanclief/agent-composer/models/messaging"
)

func TestCreateChannelArchivesAddsTable(t *testing.T) {
	ctx := context.Background()
	db := newSQLite(t)

	// Twice: the second run, as on a fresh database, changes nothing.
	for range 2 {
		err := createChannelArchives(ctx, db)
		if err != nil {
			t.Fatal(err)
		}
	}

	at := time.Date(2026, 11, 3, 0, 0, 0, 0, time.UTC)
	added, err := messaging.ArchiveChannel(ctx, db, "#fix-login", at)
	if err != nil || !added {
		t.Fatalf("archive = %v, %v", added, err)
	}

	added, err = messaging.ArchiveChannel(ctx, db, "#fix-login", at.Add(time.Hour))
	if err != nil || added {
		t.Fatalf("archive again = %v, %v", added, err)
	}

	archived, err := messaging.ArchivedChannels(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	if len(archived) != 1 || !archived["#fix-login"].Equal(at) {
		t.Fatalf("archived = %v", archived)
	}
}
