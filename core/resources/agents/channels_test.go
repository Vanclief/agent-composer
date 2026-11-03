package agents

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/vanclief/agent-composer/models/agent"
	msgmodels "github.com/vanclief/agent-composer/models/messaging"
)

// acceptDMs is messaging's check once direct-message channels exist
// (ADR 0011): @<owner>/<name> passes as well as # names.
func acceptDMs(channel string) error {
	if strings.HasPrefix(channel, "@") && strings.Count(channel, "/") == 1 {
		return nil
	}

	return msgmodels.ValidateChannel(channel)
}

func TestReadsHasTheDMChannelOnlyWhenMessagingAcceptsIt(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	env.create(t, "watcher")

	record, err := agent.GetAgentByName(ctx, env.db, "watcher-1")
	if err != nil {
		t.Fatal(err)
	}
	if record.Owner == "" {
		t.Fatal("the agent has no owner")
	}
	tmpl, err := env.api.template(record.Role)
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name      string
		validator func(string) error
		want      []string
	}{
		{"messaging rejects @ names", msgmodels.ValidateChannel, []string{"#ci-*"}},
		{"messaging knows DM channels", acceptDMs, []string{"@franco/watcher-1", "#ci-*"}},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			validChannel = c.validator
			defer func() {
				validChannel = msgmodels.ValidateChannel
			}()

			reads, err := env.api.Reads(ctx, record, tmpl)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(reads, c.want) {
				t.Fatalf("reads = %q, want %q", reads, c.want)
			}

			for _, pattern := range reads {
				if isGlob(pattern) {
					continue
				}
				err = c.validator(pattern)
				if err != nil {
					t.Fatalf("reads holds %q, which messaging rejects: %v", pattern, err)
				}
			}
		})
	}
}
