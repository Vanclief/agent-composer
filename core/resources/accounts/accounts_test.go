package accounts

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/sqlitedialect"
	"github.com/vanclief/ez"
	_ "modernc.org/sqlite"

	"github.com/vanclief/agent-composer/models/account"
)

// A made-up value: no test holds a real credential.
const value = "made-up-token"

func newTestAPI(t *testing.T, users SecretUsers) (*API, string) {
	t.Helper()

	sqldb, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	sqldb.SetMaxOpenConns(1)
	t.Cleanup(func() {
		_ = sqldb.Close()
	})

	db := bun.NewDB(sqldb, sqlitedialect.New())
	_, err = db.NewCreateTable().Model((*account.Account)(nil)).Exec(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	// No secrets file yet: Create makes it.
	secretsPath := filepath.Join(t.TempDir(), "config", "secrets.env")

	return New(db, secretsPath, users), secretsPath
}

type fakeUsers map[string][]string

func (f fakeUsers) SandboxesUsingSecret(ctx context.Context, secretName string) ([]string, error) {
	return f[secretName], nil
}

type fakeUsage struct {
	asked []string
}

func (f *fakeUsage) Usage(ctx context.Context, secretName string) (*UsageView, error) {
	f.asked = append(f.asked, secretName)
	if secretName == "ACCOUNT_BROKEN" {
		return nil, ez.New(ez.EUNAVAILABLE, "usage endpoint is down", nil)
	}

	return &UsageView{Windows: []UsageWindow{{ID: "session", Label: "Session", Used: 0.16}}}, nil
}

func TestCreateMakesTheSecretsFileOwnerOnly(t *testing.T) {
	api, secretsPath := newTestAPI(t, nil)

	view, err := api.Create(context.Background(), nil, &CreateRequest{Name: "work", Provider: account.ProviderAnthropic, Kind: account.KindOAuth, Value: " " + value + "\n"})
	if err != nil {
		t.Fatal(err)
	}
	if view.Secret != "ACCOUNT_WORK" || view.Default || view.Usage != nil {
		t.Fatalf("view = %+v", view)
	}

	info, err := os.Stat(secretsPath)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %04o", info.Mode().Perm())
	}

	data, err := os.ReadFile(secretsPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "ACCOUNT_WORK='"+value+"'\n" {
		t.Fatal("secrets.env does not hold exactly the trimmed account line")
	}
}

// A value the file cannot hold fails the create and leaves no row.
func TestCreateLeavesNoRowWhenTheLineCannotBeWritten(t *testing.T) {
	api, _ := newTestAPI(t, nil)
	ctx := context.Background()

	_, err := api.Create(ctx, nil, &CreateRequest{Name: "work", Provider: account.ProviderAnthropic, Kind: account.KindOAuth, Value: `it's "quoted"`})
	if ez.ErrorCode(err) != ez.EINVALID {
		t.Fatalf("err = %v", err)
	}
	if strings.Contains(ez.ErrorMessage(err), "quoted") {
		t.Fatal("the error spells the value")
	}

	_, err = account.GetByName(ctx, api.db, "work")
	if ez.ErrorCode(err) != ez.ENOTFOUND {
		t.Fatalf("row left behind: %v", err)
	}
}

func TestCreateRequestNeverMarshalsItsValue(t *testing.T) {
	request := &CreateRequest{Name: "work", Provider: account.ProviderAnthropic, Kind: account.KindOAuth, Value: value}

	data, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), value) || strings.Contains(request.String(), value) {
		t.Fatal("the request prints its value")
	}

	var bound CreateRequest
	err = json.Unmarshal([]byte(`{"name":"work","value":"`+value+`"}`), &bound)
	if err != nil {
		t.Fatal(err)
	}
	if bound.Value != value {
		t.Fatal("binding no longer reads the value")
	}
}

// Usage is asked only for anthropic oauth accounts; an error shows on the
// view.
func TestUsageSource(t *testing.T) {
	api, _ := newTestAPI(t, nil)
	ctx := context.Background()
	source := &fakeUsage{}
	api.SetUsage(source)

	requests := []CreateRequest{
		{Name: "work", Provider: account.ProviderAnthropic, Kind: account.KindOAuth, Value: value},
		{Name: "broken", Provider: account.ProviderAnthropic, Kind: account.KindOAuth, Value: value},
		{Name: "key", Provider: account.ProviderAnthropic, Kind: account.KindAPIKey, Value: value},
		{Name: "codex", Provider: account.ProviderOpenAI, Kind: account.KindAPIKey, Value: value},
	}
	for i := range requests {
		_, err := api.Create(ctx, nil, &requests[i])
		if err != nil {
			t.Fatal(err)
		}
	}

	source.asked = nil
	list, err := api.List(ctx, nil, &ListRequest{})
	if err != nil {
		t.Fatal(err)
	}

	usage := map[string]*UsageView{}
	for _, view := range list.Accounts {
		usage[view.Name] = view.Usage
	}
	if usage["work"] == nil || usage["work"].Windows[0].Used != 0.16 || usage["work"].Error != "" {
		t.Fatalf("work usage = %+v", usage["work"])
	}
	if usage["broken"] == nil || usage["broken"].Error != "usage endpoint is down" {
		t.Fatalf("broken usage = %+v", usage["broken"])
	}
	if usage["key"] != nil || usage["codex"] != nil {
		t.Fatal("usage asked for an account that is not anthropic oauth")
	}
	if len(source.asked) != 2 {
		t.Fatalf("asked = %q", source.asked)
	}
}

func TestDeleteRefusedWhileInUse(t *testing.T) {
	users := fakeUsers{"ACCOUNT_WORK": {"fix-1"}}
	api, secretsPath := newTestAPI(t, users)
	ctx := context.Background()

	_, err := api.Create(ctx, nil, &CreateRequest{Name: "work", Provider: account.ProviderAnthropic, Kind: account.KindOAuth, Value: value})
	if err != nil {
		t.Fatal(err)
	}

	_, err = api.Delete(ctx, nil, &DeleteRequest{Name: "work"})
	if ez.ErrorCode(err) != ez.ECONFLICT || !strings.Contains(ez.ErrorMessage(err), "fix-1") {
		t.Fatalf("err = %v", err)
	}

	delete(users, "ACCOUNT_WORK")
	_, err = api.Delete(ctx, nil, &DeleteRequest{Name: "work"})
	if err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(secretsPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) != 0 {
		t.Fatal("the line is still in secrets.env")
	}
}
