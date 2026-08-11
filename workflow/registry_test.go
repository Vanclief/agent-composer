package workflow

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/sqlitedialect"
	workflowmodels "github.com/vanclief/agent-composer/models/workflow"
	_ "modernc.org/sqlite"
)

// newTestRegistry returns a registry backed by an in-memory SQLite
// database with the workflow tables created.
func newTestRegistry(t *testing.T) (*Registry, context.Context) {
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

	return NewRegistry(db), ctx
}

// importYAML installs spec YAML into the test registry.
func importYAML(t *testing.T, ctx context.Context, registry *Registry, raw string) WorkflowSummary {
	t.Helper()

	path := filepath.Join(t.TempDir(), "spec.yaml")
	err := os.WriteFile(path, []byte(raw), 0644)
	if err != nil {
		t.Fatal(err)
	}

	summary, err := registry.ImportFile(ctx, path, false)
	if err != nil {
		t.Fatal(err)
	}

	return summary
}
