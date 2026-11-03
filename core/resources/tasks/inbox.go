package tasks

import (
	"context"

	"github.com/vanclief/ez"

	"github.com/vanclief/agent-composer/messaging"
	taskmodels "github.com/vanclief/agent-composer/models/task"
)

// channelsFor is the channels the person called name reads besides their
// inbox: #<task> for every task they manage. A task they only own is not
// one: its agent asks its manager, who raises to the owner in their inbox
// what only the owner can decide (ADR 0009 item 5). Messaging's questions
// route uses it for ?for=<name>.
func (api *API) channelsFor(ctx context.Context, name string) ([]string, error) {
	var names []string

	err := api.db.NewSelect().
		Model((*taskmodels.Task)(nil)).
		Column("name").
		Where("manager = ?", name).
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
