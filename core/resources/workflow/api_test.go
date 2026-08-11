package workflow

import (
	"context"
	"database/sql"
	"testing"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/sqlitedialect"
	workflowmodels "github.com/vanclief/agent-composer/models/workflow"
	workflowruntime "github.com/vanclief/agent-composer/workflow"
	_ "modernc.org/sqlite"
)

// newTestAPI returns an API whose registry is backed by an in-memory
// SQLite database.
func newTestAPI(t *testing.T) (*API, context.Context) {
	t.Helper()

	sqldb, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	// Every pooled connection would get its own :memory: database, so
	// the pool must stay at one connection.
	sqldb.SetMaxOpenConns(1)
	t.Cleanup(func() {
		_ = sqldb.Close()
	})

	db := bun.NewDB(sqldb, sqlitedialect.New())
	ctx := context.Background()

	tables := []interface{}{
		(*workflowmodels.Workflow)(nil),
		(*workflowmodels.WorkflowVersion)(nil),
	}
	for _, table := range tables {
		_, err = db.NewCreateTable().Model(table).Exec(ctx)
		if err != nil {
			t.Fatal(err)
		}
	}

	api := &API{
		Registry: workflowruntime.NewRegistry(db),
		db:       db,
	}

	return api, ctx
}

// installSpec inserts an installed workflow row directly — these
// tests exercise reads, not the import pipeline.
func installSpec(t *testing.T, ctx context.Context, api *API, slug, spec string) {
	t.Helper()

	record := &workflowmodels.Workflow{
		Slug:    slug,
		Version: 1,
		Spec:    spec,
	}

	err := record.Insert(ctx, api.db)
	if err != nil {
		t.Fatal(err)
	}
}
