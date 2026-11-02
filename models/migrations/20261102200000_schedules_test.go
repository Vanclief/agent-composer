package migrations

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/vanclief/agent-composer/models/schedule"
)

func TestCreateSchedulesAddsTables(t *testing.T) {
	ctx := context.Background()
	db := newSQLite(t)

	// Twice: the second run, as on a fresh database, changes nothing.
	for range 2 {
		err := createSchedules(ctx, db)
		if err != nil {
			t.Fatal(err)
		}
	}

	record := &schedule.Schedule{
		Name:    "pr-reviews",
		Every:   "2m",
		Enabled: false,
		Task:    schedule.Template{Title: "Review {{.title}}", Accept: []string{"posted"}},
		Poll:    &schedule.Poll{Command: "echo", Secrets: []string{"GITHUB_TOKEN"}},
	}
	err := record.Insert(ctx, db)
	if err != nil {
		t.Fatal(err)
	}

	read, err := schedule.GetByName(ctx, db, "pr-reviews")
	if err != nil {
		t.Fatal(err)
	}
	if read.Enabled || read.Task.Title != "Review {{.title}}" || read.Poll == nil || read.Poll.Secrets[0] != "GITHUB_TOKEN" || read.LastRunAt != nil {
		t.Fatalf("schedule = %+v", read)
	}

	item := &schedule.Item{ScheduleID: record.ID, Key: "pr-1@abc", TaskID: uuid.New(), Task: "pr-reviews-1"}
	err = item.Insert(ctx, db)
	if err != nil {
		t.Fatal(err)
	}

	again := &schedule.Item{ScheduleID: record.ID, Key: "pr-1@abc", TaskID: uuid.New(), Task: "pr-reviews-2"}
	err = again.Insert(ctx, db)
	if err == nil {
		t.Fatal("a second item with the same key was inserted")
	}

	seen, err := schedule.SeenKeys(ctx, db, record.ID, []string{"pr-1@abc", "pr-2@def"})
	if err != nil {
		t.Fatal(err)
	}
	if !seen["pr-1@abc"] || seen["pr-2@def"] {
		t.Fatalf("seen = %v", seen)
	}
}
