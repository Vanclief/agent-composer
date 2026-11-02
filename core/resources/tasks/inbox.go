package tasks

import (
	"context"

	"github.com/uptrace/bun"
	"github.com/vanclief/ez"

	"github.com/vanclief/agent-composer/messaging"
	taskmodels "github.com/vanclief/agent-composer/models/task"
)

// channelsFor is the channels the person called name reads besides their
// inbox: #<task> for every task they manage or own. Messaging's questions
// route uses it for ?for=<name>.
func (api *API) channelsFor(ctx context.Context, name string) ([]string, error) {
	var names []string

	err := api.db.NewSelect().
		Model((*taskmodels.Task)(nil)).
		Column("name").
		WhereGroup(" AND ", func(q *bun.SelectQuery) *bun.SelectQuery {
			return q.Where("manager = ?", name).WhereOr("owner = ?", name)
		}).
		Order("name ASC").
		Scan(ctx, &names)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	channels := make([]string, 0, len(names))
	for _, taskName := range names {
		channels = append(channels, messaging.TaskChannel(taskName))
	}

	return channels, nil
}
