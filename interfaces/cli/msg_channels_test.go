package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/sqlitedialect"

	"github.com/vanclief/agent-composer/messaging"
	"github.com/vanclief/agent-composer/messaging/httpapi"
	msgmodels "github.com/vanclief/agent-composer/models/messaging"
)

// captureStdout runs fn and answers what it printed with fmt.Println.
func captureStdout(t *testing.T, fn func() error) (string, error) {
	t.Helper()

	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}

	saved := os.Stdout
	os.Stdout = writer
	runErr := fn()
	os.Stdout = saved

	err = writer.Close()
	if err != nil {
		t.Fatal(err)
	}

	out, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}

	return string(out), runErr
}

// agc msg channels leaves archived channels out; --archived lists them
// too, with archived_at.
func TestMsgChannelsArchived(t *testing.T) {
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
	for _, table := range []interface{}{(*msgmodels.Participant)(nil), (*msgmodels.Message)(nil), (*msgmodels.ChannelArchive)(nil)} {
		_, err = db.NewCreateTable().Model(table).Exec(ctx)
		if err != nil {
			t.Fatal(err)
		}
	}

	svc := messaging.NewService(db)
	for _, channel := range []string{"#old-done", "#open-task"} {
		_, err = svc.PostAsSystem(ctx, channel, "Opening")
		if err != nil {
			t.Fatal(err)
		}
	}

	at := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	_, err = svc.Archive(ctx, "#old-done", at)
	if err != nil {
		t.Fatal(err)
	}

	human := func(*http.Request) (messaging.Caller, error) {
		return messaging.Caller{Human: true}, nil
	}
	server := httptest.NewServer(httpapi.New(svc, human))
	t.Cleanup(server.Close)
	t.Setenv("AGC_MSG_URL", server.URL)

	list := func(args ...string) httpapi.ChannelsResponse {
		out, err := captureStdout(t, func() error {
			_, err := runAgc(t, append([]string{"msg", "channels"}, args...)...)
			return err
		})
		if err != nil {
			t.Fatal(err)
		}

		var res httpapi.ChannelsResponse
		err = json.Unmarshal([]byte(out), &res)
		if err != nil {
			t.Fatalf("%v in %q", err, out)
		}

		return res
	}

	names := func(res httpapi.ChannelsResponse) []string {
		got := []string{}
		for _, info := range res.Channels {
			got = append(got, info.Channel)
		}

		return got
	}

	plain := list()
	if !reflect.DeepEqual(names(plain), []string{"#open-task"}) {
		t.Fatalf("msg channels listed %q", names(plain))
	}

	archived := list("--archived")
	if !reflect.DeepEqual(names(archived), []string{"#old-done", "#open-task"}) {
		t.Fatalf("msg channels --archived listed %q", names(archived))
	}
	if archived.Channels[0].ArchivedAt == nil || !archived.Channels[0].ArchivedAt.Equal(at) || archived.Channels[1].ArchivedAt != nil {
		t.Fatalf("archived_at %+v", archived.Channels)
	}
}
