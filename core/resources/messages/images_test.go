package messages

import (
	"context"
	"database/sql"
	"io"
	"path/filepath"
	"testing"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/sqlitedialect"
	"github.com/vanclief/ez"
	_ "modernc.org/sqlite"

	"github.com/vanclief/agent-composer/messaging"
	msgmodels "github.com/vanclief/agent-composer/models/messaging"
)

func TestOpenImage(t *testing.T) {
	ctx := context.Background()

	sqldb, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	sqldb.SetMaxOpenConns(1)
	t.Cleanup(func() {
		_ = sqldb.Close()
	})

	db := bun.NewDB(sqldb, sqlitedialect.New())
	for _, table := range []interface{}{(*msgmodels.Participant)(nil), (*msgmodels.Message)(nil), (*msgmodels.ChannelArchive)(nil)} {
		_, err = db.NewCreateTable().Model(table).Exec(ctx)
		if err != nil {
			t.Fatal(err)
		}
	}

	svc := messaging.NewService(db)
	svc.SetImagesDir(filepath.Join(t.TempDir(), "images"))
	api := NewAPI(svc)

	human := messaging.Caller{Human: true}
	p, err := svc.Join(ctx, human, "franco")
	if err != nil {
		t.Fatal(err)
	}

	data := []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR shot")
	m, err := svc.PostImages(ctx, human, p.ID, "#talk", "look", nil, "", []msgmodels.ImageUpload{{Name: "shot.png", Data: data}})
	if err != nil {
		t.Fatal(err)
	}

	opened, err := api.OpenImage(ctx, nil, &GetImageRequest{MessageID: m.ID, Name: "shot.png"})
	if err != nil {
		t.Fatal(err)
	}
	defer opened.File.Close()

	image := opened.Image
	got, err := io.ReadAll(opened.File)
	if err != nil || string(got) != string(data) || image.Type != "image/png" {
		t.Fatalf("image %+v, %q, %v", image, got, err)
	}

	cases := []GetImageRequest{{MessageID: 0, Name: "shot.png"}, {MessageID: m.ID, Name: ""}, {MessageID: m.ID, Name: "nope.png"}, {MessageID: 99, Name: "shot.png"}}
	for _, request := range cases {
		_, err = api.OpenImage(ctx, nil, &request)
		if ez.ErrorCode(err) != ez.ENOTFOUND {
			t.Errorf("%+v: err = %v, want not found", request, err)
		}
	}
}
