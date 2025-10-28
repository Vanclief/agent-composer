package hook

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
	"github.com/vanclief/ez"
)

func GetHookByID(ctx context.Context, db bun.IDB, id uuid.UUID) (*Hook, error) {
	const op = "hook.GetHookByID"

	h := new(Hook)
	err := db.NewSelect().
		Model(h).
		Where("id = ?", id).
		Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			errMsg := fmt.Sprintf("hook with ID %s not found", id)
			return nil, ez.New(op, ez.ENOTFOUND, errMsg, err)
		}
		return nil, ez.Wrap(op, err)
	}

	return h, nil
}
