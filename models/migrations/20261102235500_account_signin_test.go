package migrations

import (
	"context"
	"testing"
	"time"

	"github.com/vanclief/agent-composer/models/account"
)

func TestAccountSigninAddsColumn(t *testing.T) {
	ctx := context.Background()
	db := newSQLite(t)

	// The accounts table as the accounts migration first made it.
	_, err := db.ExecContext(ctx, `CREATE TABLE accounts (id TEXT PRIMARY KEY, name TEXT NOT NULL UNIQUE,
provider TEXT NOT NULL, kind TEXT NOT NULL, secret TEXT NOT NULL, email TEXT NOT NULL DEFAULT '',
is_default BOOLEAN NOT NULL DEFAULT FALSE, created_at TIMESTAMP NOT NULL, updated_at TIMESTAMP NOT NULL)`)
	if err != nil {
		t.Fatal(err)
	}

	now := time.Date(2026, 11, 2, 23, 0, 0, 0, time.UTC)
	_, err = db.ExecContext(ctx, "INSERT INTO accounts (id, name, provider, kind, secret, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)",
		"0190a7c2-0000-7000-8000-000000000001", "work", "anthropic", "oauth", "ACCOUNT_WORK", now, now)
	if err != nil {
		t.Fatal(err)
	}

	// Twice: the second run, as on a fresh database, changes nothing.
	for range 2 {
		err = accountSignin(ctx, db)
		if err != nil {
			t.Fatal(err)
		}
	}

	record, err := account.GetByName(ctx, db, "work")
	if err != nil {
		t.Fatal(err)
	}
	if record.SignedInAt != nil {
		t.Fatalf("signed_in_at = %v", record.SignedInAt)
	}

	err = record.SetSignedIn(ctx, db, &now, "franco@example.com")
	if err != nil {
		t.Fatal(err)
	}

	record, err = account.GetByName(ctx, db, "work")
	if err != nil {
		t.Fatal(err)
	}
	if record.SignedInAt == nil || !record.SignedInAt.Equal(now) || record.Email != "franco@example.com" {
		t.Fatalf("account = %+v", record)
	}
}
