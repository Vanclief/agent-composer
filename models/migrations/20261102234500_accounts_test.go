package migrations

import (
	"context"
	"testing"

	"github.com/vanclief/agent-composer/models/account"
)

func TestCreateAccountsAllowsOneDefaultPerProvider(t *testing.T) {
	ctx := context.Background()
	db := newSQLite(t)

	// Twice: the second run, as on a fresh database, changes nothing.
	for range 2 {
		err := createAccounts(ctx, db)
		if err != nil {
			t.Fatal(err)
		}
	}

	work := &account.Account{Name: "work", Provider: account.ProviderAnthropic, Kind: account.KindOAuth, Secret: account.SecretName("work"), Default: true}
	err := work.Insert(ctx, db)
	if err != nil {
		t.Fatal(err)
	}

	again := &account.Account{Name: "work", Provider: account.ProviderOpenAI, Kind: account.KindAPIKey, Secret: account.SecretName("work")}
	err = again.Insert(ctx, db)
	if err == nil {
		t.Fatal("a second account named work was inserted")
	}

	home := &account.Account{Name: "home", Provider: account.ProviderAnthropic, Kind: account.KindOAuth, Secret: account.SecretName("home"), Default: true}
	err = home.Insert(ctx, db)
	if err == nil {
		t.Fatal("a second anthropic default was inserted")
	}

	openai := &account.Account{Name: "codex", Provider: account.ProviderOpenAI, Kind: account.KindAPIKey, Secret: account.SecretName("codex"), Default: true}
	err = openai.Insert(ctx, db)
	if err != nil {
		t.Fatal(err)
	}

	home.Default = false
	home.ID = [16]byte{}
	err = home.Insert(ctx, db)
	if err != nil {
		t.Fatal(err)
	}

	err = home.SetDefault(ctx, db, true)
	if err != nil {
		t.Fatal(err)
	}

	read, err := account.GetDefault(ctx, db, account.ProviderAnthropic)
	if err != nil {
		t.Fatal(err)
	}
	if read == nil || read.Name != "home" {
		t.Fatalf("anthropic default = %+v", read)
	}
}
