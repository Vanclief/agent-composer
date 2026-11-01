package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect"
	"github.com/vanclief/ez"
)

// Tasks gained a title. A database created before that keeps its old tasks
// table, so the column is added here. A fresh database already has it from
// the model, which is why the column is looked up first.
func init() {
	Migrations.MustRegister(addTasksTitle, func(ctx context.Context, db *bun.DB) error {
		// Never rolled back: we do not roll back schema, and older SQLite
		// versions cannot drop a column.
		return nil
	})
}

func addTasksTitle(ctx context.Context, db *bun.DB) error {
	exists, err := columnExists(ctx, db, "tasks", "title")
	if err != nil {
		return ez.Wrap(err)
	}

	if exists {
		return nil
	}

	_, err = db.ExecContext(ctx, "ALTER TABLE tasks ADD COLUMN title TEXT NOT NULL DEFAULT ''")
	if err != nil {
		return ez.Wrap(err)
	}

	return nil
}

// columnExists reports whether table has column, asking the database its
// own way.
func columnExists(ctx context.Context, db *bun.DB, table, column string) (bool, error) {
	var query string

	switch db.Dialect().Name() {
	case dialect.SQLite:
		query = "SELECT count(*) FROM pragma_table_info(?) WHERE name = ?"
	case dialect.PG:
		query = "SELECT count(*) FROM information_schema.columns WHERE table_schema = current_schema() AND table_name = ? AND column_name = ?"
	default:
		msg := fmt.Sprintf("Migrations do not support the %s dialect", db.Dialect().Name())
		return false, ez.New(ez.EINVALID, msg, nil)
	}

	var count int

	err := db.QueryRowContext(ctx, query, table, column).Scan(&count)
	if err != nil {
		return false, ez.Wrap(err)
	}

	return count > 0, nil
}
