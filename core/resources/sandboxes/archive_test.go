package sandboxes

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/vanclief/ez"

	"github.com/vanclief/agent-composer/core/events"
	"github.com/vanclief/agent-composer/messaging"
)

// archiveEnv has, with the clock at now: old-done (done 8 days ago),
// fresh-done (done a day ago) and open-task (not done), each with agc's
// opening message in its channel, keep_done off and archive_done 7 days.
func archiveEnv(t *testing.T) (*testEnv, time.Time) {
	t.Helper()

	env := newTestEnv(t)
	now := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	env.api.now = func() time.Time {
		return now
	}
	env.api.keepDone = 0
	env.api.archiveDone = 7 * 24 * time.Hour

	insertDone(t, env, "old-done", now.Add(-8*24*time.Hour))
	insertDone(t, env, "fresh-done", now.Add(-24*time.Hour))
	insertDone(t, env, "open-task", time.Time{})

	for _, name := range []string{"old-done", "fresh-done", "open-task"} {
		_, err := env.api.msg.PostAsSystem(context.Background(), messaging.TaskChannel(name), "Opening "+name)
		if err != nil {
			t.Fatal(err)
		}
	}

	return env, now
}

func channelNames(infos []messaging.ChannelInfo) []string {
	names := []string{}
	for _, info := range infos {
		names = append(names, info.Channel)
	}

	return names
}

func TestTickArchivesChannelsOfTasksDoneLongAgo(t *testing.T) {
	env, now := archiveEnv(t)
	ctx := context.Background()

	bus := events.New()
	env.api.msg.SetEvents(bus)
	feedCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	feed := bus.Subscribe(feedCtx)

	// Twice: the second tick finds nothing new to archive.
	env.api.tick(ctx)
	env.api.tick(ctx)

	archivedEvents := []messaging.ArchivedEvent{}
	for len(feed) > 0 {
		event := <-feed
		if event.Type == events.ChannelArchived {
			archivedEvents = append(archivedEvents, event.Data.(messaging.ArchivedEvent))
		}
	}
	want := []messaging.ArchivedEvent{{Channel: "#old-done", ArchivedAt: now}}
	if !reflect.DeepEqual(archivedEvents, want) {
		t.Fatalf("channel.archived %+v", archivedEvents)
	}

	human := messaging.Caller{Human: true}

	// Listed only when asked for, then with archived_at.
	listed, err := env.api.msg.Channels(ctx, human, false)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(channelNames(listed), []string{"#fresh-done", "#open-task"}) {
		t.Fatalf("listed %q", channelNames(listed))
	}

	all, err := env.api.msg.Channels(ctx, human, true)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(channelNames(all), []string{"#fresh-done", "#old-done", "#open-task"}) {
		t.Fatalf("listed with archived %q", channelNames(all))
	}
	if all[1].ArchivedAt == nil || !all[1].ArchivedAt.Equal(now) || all[0].ArchivedAt != nil || all[1].Count != 1 {
		t.Fatalf("archived_at %+v", all)
	}

	// Still readable.
	read, err := env.api.msg.Read(ctx, human, "#old-done", 0, 0)
	if err != nil || len(read) != 1 || read[0].Body != "Opening old-done" {
		t.Fatalf("read %+v, %v", read, err)
	}

	// Closed to posts, from people and from the task itself, but not to agc.
	franco, err := env.api.msg.Join(ctx, human, "franco")
	if err != nil {
		t.Fatal(err)
	}
	_, err = env.api.msg.Post(ctx, human, franco.ID, "#old-done", "Still there?", nil, "")
	if ez.ErrorCode(err) != ez.ENOTAUTHORIZED || ez.ErrorMessage(err) != "channel archived" {
		t.Fatalf("post by a person: %v", err)
	}

	sandbox := messaging.Caller{Task: "old-done"}
	agent, err := env.api.msg.Join(ctx, sandbox, "old-done")
	if err != nil {
		t.Fatal(err)
	}
	_, err = env.api.msg.Post(ctx, sandbox, agent.ID, "#old-done", "Late", nil, "")
	if ez.ErrorCode(err) != ez.ENOTAUTHORIZED {
		t.Fatalf("post by the task: %v", err)
	}

	_, err = env.api.msg.Post(ctx, human, franco.ID, "#fresh-done", "Fine here", nil, "")
	if err != nil {
		t.Fatalf("post in a channel not archived: %v", err)
	}

	_, err = env.api.msg.PostAsSystem(ctx, "#old-done", "agc may still post")
	if err != nil {
		t.Fatalf("post by agc: %v", err)
	}
}

// The clock moves on: a task archived later is archived then, never
// before, and archive_done 0 leaves every channel alone.
func TestTickArchivesWhenTheClockPassesArchiveDone(t *testing.T) {
	env, now := archiveEnv(t)
	ctx := context.Background()
	human := messaging.Caller{Human: true}

	env.api.archiveDone = 0
	env.api.tick(ctx)
	listed, err := env.api.msg.Channels(ctx, human, false)
	if err != nil || len(listed) != 3 {
		t.Fatalf("archive_done 0 listed %+v, %v", listed, err)
	}

	env.api.archiveDone = 7 * 24 * time.Hour
	later := now.Add(6*24*time.Hour + time.Minute)
	env.api.now = func() time.Time {
		return later
	}
	env.api.tick(ctx)

	listed, err = env.api.msg.Channels(ctx, human, false)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(channelNames(listed), []string{"#open-task"}) {
		t.Fatalf("listed %q", channelNames(listed))
	}
}
