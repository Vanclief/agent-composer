package controller

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/uptrace/bun/migrate"
	_ "modernc.org/sqlite"

	taskmodels "github.com/vanclief/agent-composer/models/task"
)

// oldTasksTable is the tasks table as it was before tasks had a title.
const oldTasksTable = `CREATE TABLE tasks (
	id TEXT NOT NULL PRIMARY KEY,
	name VARCHAR NOT NULL UNIQUE,
	spec BLOB,
	created_at TIMESTAMP NOT NULL DEFAULT current_timestamp,
	updated_at TIMESTAMP NOT NULL DEFAULT current_timestamp
)`

// setupAt runs the startup database path against the SQLite file at path.
func setupAt(t *testing.T, path string) *Controller {
	t.Helper()

	controller := &Controller{}
	controller.Config.SQLite.Path = path

	err := controller.Setup()
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_ = controller.DB.Close()
	})

	return controller
}

func countTitleColumns(t *testing.T, controller *Controller) int {
	t.Helper()

	var count int

	err := controller.DB.QueryRowContext(context.Background(), "SELECT count(*) FROM pragma_table_info('tasks') WHERE name = 'title'").Scan(&count)
	if err != nil {
		t.Fatal(err)
	}

	return count
}

func countAppliedMigrations(t *testing.T, controller *Controller) int {
	t.Helper()

	var count int

	err := controller.DB.QueryRowContext(context.Background(), "SELECT count(*) FROM bun_migrations").Scan(&count)
	if err != nil {
		t.Fatal(err)
	}

	return count
}

func TestSetupAddsTitleToOldTasksTable(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "agc.db")

	sqldb, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}

	_, err = sqldb.ExecContext(ctx, oldTasksTable)
	if err != nil {
		t.Fatal(err)
	}

	_, err = sqldb.ExecContext(ctx, "INSERT INTO tasks (id, name, spec) VALUES ('0192f0c4-0000-7000-8000-000000000001', 'old-task', '{}')")
	if err != nil {
		t.Fatal(err)
	}

	err = sqldb.Close()
	if err != nil {
		t.Fatal(err)
	}

	controller := setupAt(t, path)

	if countTitleColumns(t, controller) != 1 {
		t.Fatal("tasks has no title column after startup")
	}

	var oldTitle string

	err = controller.DB.QueryRowContext(ctx, "SELECT title FROM tasks WHERE name = 'old-task'").Scan(&oldTitle)
	if err != nil {
		t.Fatal(err)
	}

	if oldTitle != "" {
		t.Fatalf("existing task title = %q, want empty", oldTitle)
	}

	// ADR 0008 gave the old task's spec to a sandbox of the same name.
	sandbox, err := taskmodels.GetSandboxByName(ctx, controller.DB, "old-task")
	if err != nil {
		t.Fatal(err)
	}
	if string(sandbox.Spec) != "{}" || sandbox.BootedAt == nil {
		t.Fatalf("migrated sandbox = %+v", sandbox)
	}

	task := &taskmodels.Task{
		Title: "Review facebook ads",
		Name:  "review-facebook-ads",
		Brief: "Find the ads that lose money.",
	}

	err = task.Insert(ctx, controller.DB)
	if err != nil {
		t.Fatal(err)
	}

	stored := &taskmodels.Task{}

	err = controller.DB.NewSelect().Model(stored).Where("name = ?", task.Name).Scan(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if stored.Title != task.Title {
		t.Fatalf("stored title = %q, want %q", stored.Title, task.Title)
	}
}

func TestSetupOnFreshDatabaseKeepsSingleTitleColumn(t *testing.T) {
	controller := setupAt(t, filepath.Join(t.TempDir(), "agc.db"))

	if countTitleColumns(t, controller) != 1 {
		t.Fatal("fresh tasks table should have exactly one title column")
	}

	if countAppliedMigrations(t, controller) != 5 {
		t.Fatal("the title, sandboxes, task agents, task acceptance and schedules migrations should be recorded as applied")
	}
}

func TestSetupTwiceIsNoOp(t *testing.T) {
	path := filepath.Join(t.TempDir(), "agc.db")

	first := setupAt(t, path)
	applied := countAppliedMigrations(t, first)

	err := first.DB.Close()
	if err != nil {
		t.Fatal(err)
	}

	second := setupAt(t, path)

	if countAppliedMigrations(t, second) != applied {
		t.Fatal("a second startup applied migrations again")
	}

	if countTitleColumns(t, second) != 1 {
		t.Fatal("tasks should still have exactly one title column")
	}
}

func TestRunMigrationsFailureStopsAndRetries(t *testing.T) {
	ctx := context.Background()
	controller := setupAt(t, filepath.Join(t.TempDir(), "agc.db"))

	set := migrate.NewMigrations()
	set.Add(migrate.Migration{
		Name:    "20990101000000",
		Comment: "broken",
		Up: func(ctx context.Context, migrator *migrate.Migrator, migration *migrate.Migration) error {
			return errors.New("boom")
		},
	})

	err := runMigrations(ctx, controller.DB.DB, set)
	if err == nil {
		t.Fatal("a failing migration should fail startup")
	}

	if !strings.Contains(err.Error(), "Database migration failed") || !strings.Contains(err.Error(), "20990101000000") || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("error should name the failed migration and its cause, got %q", err.Error())
	}

	var marked int

	err = controller.DB.QueryRowContext(ctx, "SELECT count(*) FROM bun_migrations WHERE name = '20990101000000'").Scan(&marked)
	if err != nil {
		t.Fatal(err)
	}

	if marked != 0 {
		t.Fatal("a failed migration must not be marked applied, so the next start retries it")
	}
}
