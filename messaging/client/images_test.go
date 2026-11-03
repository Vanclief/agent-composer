package client

import (
	"bytes"
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/sqlitedialect"
	"github.com/vanclief/ez"
	_ "modernc.org/sqlite"

	"github.com/vanclief/agent-composer/messaging"
	"github.com/vanclief/agent-composer/messaging/httpapi"
	msgmodels "github.com/vanclief/agent-composer/models/messaging"
)

// pngHeader is enough of a PNG for the type sniffing.
var pngHeader = []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR")

// ADR 0005 "Images": a sandbox posts and fetches images over its socket,
// only in the channels it may read.
func TestClientImages(t *testing.T) {
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

	svc := messaging.NewService(db)
	svc.SetImagesDir(filepath.Join(t.TempDir(), "images"))

	task := func(*http.Request) (messaging.Caller, error) {
		return messaging.Caller{Task: "review-42"}, nil
	}
	server := httptest.NewServer(httpapi.New(svc, task))
	t.Cleanup(server.Close)

	c := &Client{BaseURL: server.URL}

	p, err := c.Join(ctx, "reviewer")
	if err != nil {
		t.Fatal(err)
	}

	shot := append(append([]byte{}, pngHeader...), []byte("shot")...)

	m, err := c.PostImages(ctx, p.ID, "#review-42", "", nil, []msgmodels.ImageUpload{{Name: "diff view.png", Data: shot}})
	if err != nil {
		t.Fatal(err)
	}

	if len(m.Images) != 1 || m.Images[0].Name != "diff-view.png" || m.Images[0].URL == "" {
		t.Fatalf("images = %+v", m.Images)
	}

	got, err := c.Image(ctx, m.ID, "diff-view.png")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, shot) {
		t.Fatalf("image = %q", got)
	}

	// A person's image in a channel the sandbox may not read.
	human := messaging.Caller{Human: true}
	franco, err := svc.Join(ctx, human, "franco")
	if err != nil {
		t.Fatal(err)
	}

	private, err := svc.PostImages(ctx, human, franco.ID, "#elsewhere", "mine", nil, "", []msgmodels.ImageUpload{{Name: "a.png", Data: shot}})
	if err != nil {
		t.Fatal(err)
	}

	_, err = c.Image(ctx, private.ID, "a.png")
	if ez.ErrorCode(err) != ez.ENOTAUTHORIZED {
		t.Fatalf("err = %v, want not authorized", err)
	}
}
