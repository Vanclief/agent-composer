package watchdog

import (
	"context"
	"database/sql"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/sqlitedialect"
	_ "modernc.org/sqlite"

	"github.com/vanclief/agent-composer/core/events"
	"github.com/vanclief/agent-composer/core/resources/sandboxes"
	"github.com/vanclief/agent-composer/messaging"
	msgmodels "github.com/vanclief/agent-composer/models/messaging"
	taskmodels "github.com/vanclief/agent-composer/models/task"
)

type fakeProbe struct {
	running bool
	job     *sandboxes.AgentJob
}

func (p *fakeProbe) Running(ctx context.Context, name string) (bool, error) {
	return p.running, nil
}

func (p *fakeProbe) AgentJob(ctx context.Context, record *taskmodels.Sandbox) (*sandboxes.AgentJob, error) {
	return p.job, nil
}

func newTestDB(t *testing.T) *bun.DB {
	t.Helper()

	sqldb, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	sqldb.SetMaxOpenConns(1)
	t.Cleanup(func() {
		_ = sqldb.Close()
	})

	db := bun.NewDB(sqldb, sqlitedialect.New())
	tables := []interface{}{
		(*taskmodels.Task)(nil),
		(*taskmodels.Sandbox)(nil),
		(*msgmodels.Participant)(nil),
		(*msgmodels.Message)(nil),
	}
	for _, table := range tables {
		_, err = db.NewCreateTable().Model(table).Exec(context.Background())
		if err != nil {
			t.Fatal(err)
		}
	}

	return db
}

type env struct {
	db    *bun.DB
	msg   *messaging.Service
	probe *fakeProbe
	dog   *Watchdog
	task  *taskmodels.Task
	box   *taskmodels.Sandbox
	clock time.Time
	feed  <-chan events.Event
}

// start is the boot time of every test; it is just before the wall clock
// because messaging stamps posts with the wall clock.
var start time.Time

func newEnv(t *testing.T) *env {
	t.Helper()

	ctx := context.Background()
	db := newTestDB(t)
	msg := messaging.NewService(db)
	start = time.Now().UTC().Add(-25 * time.Minute)

	record := &taskmodels.Task{
		ID:       uuid.New(),
		Title:    "DMs over WhatsApp",
		Name:     "dms-whatsapp",
		Assignee: "claude_code",
		Manager:  "orchestrator",
		Owner:    "franco",
	}
	err := record.Insert(ctx, db)
	if err != nil {
		t.Fatal(err)
	}

	booted := start
	box := &taskmodels.Sandbox{
		ID:       uuid.New(),
		TaskID:   record.ID,
		Name:     "dms-whatsapp",
		Spec:     []byte(`{}`),
		Command:  "agc agent run --task dms-whatsapp",
		BootedAt: &booted,
	}
	err = box.Insert(ctx, db)
	if err != nil {
		t.Fatal(err)
	}

	probe := &fakeProbe{running: true, job: &sandboxes.AgentJob{ID: "job_1", Running: true, StartedAt: start, LastOutputAt: start, LastLine: ">"}}
	bus := events.New()
	feedCtx, cancel := context.WithCancel(ctx)
	t.Cleanup(cancel)

	e := &env{db: db, msg: msg, probe: probe, task: record, box: box, clock: start, feed: bus.Subscribe(feedCtx)}
	e.dog = New(db, msg, probe, bus, Settings{Silence: 15 * time.Minute, Review: 2 * time.Hour})
	e.dog.now = func() time.Time {
		return e.clock
	}

	return e
}

// at moves the clock to start+d and runs one check.
func (e *env) at(t *testing.T, d time.Duration) {
	t.Helper()

	e.clock = start.Add(d)

	err := e.dog.Check(context.Background())
	if err != nil {
		t.Fatal(err)
	}
}

// posts lists agc's messages in channel, oldest first.
func (e *env) posts(t *testing.T, channel string) []string {
	t.Helper()

	messages := []msgmodels.Message{}
	err := e.db.NewSelect().Model(&messages).Where("channel = ?", channel).Where(`"from" = ?`, msgmodels.SystemName).Order("id ASC").Scan(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	bodies := []string{}
	for _, message := range messages {
		bodies = append(bodies, message.Body)
	}

	return bodies
}

func (e *env) events() []AlertEvent {
	got := []AlertEvent{}
	for {
		select {
		case event := <-e.feed:
			if event.Type == EventAlert {
				got = append(got, event.Data.(AlertEvent))
			}
		default:
			return got
		}
	}
}

func TestSilentAgentIsReportedHourly(t *testing.T) {
	e := newEnv(t)

	e.at(t, 10*time.Minute)
	if len(e.posts(t, "#dms-whatsapp")) != 0 {
		t.Fatalf("posted before the silence threshold")
	}

	e.at(t, 52*time.Minute)
	want := "dms-whatsapp has been silent for 52 minutes. Last terminal line: `>`"
	got := e.posts(t, "#dms-whatsapp")
	if len(got) != 1 || got[0] != want {
		t.Fatalf("#dms-whatsapp = %q, want [%q]", got, want)
	}
	inbox := e.posts(t, "#franco")
	if len(inbox) != 1 || inbox[0] != want {
		t.Fatalf("#franco = %q, want [%q]", inbox, want)
	}
	evs := e.events()
	if len(evs) != 1 || evs[0].Kind != KindSilent || evs[0].Cleared {
		t.Fatalf("events = %+v, want one silent alert", evs)
	}

	e.at(t, 90*time.Minute)
	if len(e.posts(t, "#dms-whatsapp")) != 1 {
		t.Fatalf("posted again within the hour")
	}

	e.at(t, 113*time.Minute)
	got = e.posts(t, "#dms-whatsapp")
	if len(got) != 2 || !strings.HasPrefix(got[1], "dms-whatsapp has been silent for 113 minutes.") {
		t.Fatalf("#dms-whatsapp = %q, want a second post after an hour", got)
	}
}

func TestSilenceClearsOnOutputAndRecursAtOnce(t *testing.T) {
	e := newEnv(t)

	e.at(t, 20*time.Minute)
	if len(e.posts(t, "#dms-whatsapp")) != 1 {
		t.Fatalf("want one post")
	}
	e.events()

	e.probe.job.LastOutputAt = start.Add(21 * time.Minute)
	e.at(t, 22*time.Minute)
	evs := e.events()
	if len(evs) != 1 || !evs[0].Cleared {
		t.Fatalf("events = %+v, want one clear", evs)
	}

	// Quiet again: a new condition, posted without waiting for the hour.
	e.at(t, 40*time.Minute)
	if len(e.posts(t, "#dms-whatsapp")) != 2 {
		t.Fatalf("a recurrence was not posted")
	}
}

func TestAgentWaitingOnItsManagerIsNotSilent(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()

	agent, err := e.msg.Join(ctx, messaging.Caller{Task: "dms-whatsapp"}, "dms-whatsapp")
	if err != nil {
		t.Fatal(err)
	}
	_, err = e.msg.Post(ctx, messaging.Caller{Task: "dms-whatsapp"}, agent.ID, "#dms-whatsapp", "Here is the plan.", nil, msgmodels.KindMessage)
	if err != nil {
		t.Fatal(err)
	}

	e.at(t, 24*time.Hour)
	if len(e.posts(t, "#dms-whatsapp")) != 0 {
		t.Fatalf("reported an agent whose answer waits on its manager")
	}
}

func TestRestartDoesNotRepeatWithinTheHour(t *testing.T) {
	e := newEnv(t)

	e.at(t, 20*time.Minute)

	restarted := New(e.db, e.msg, e.probe, nil, Settings{Silence: 15 * time.Minute, Review: 2 * time.Hour})
	restarted.now = e.dog.now
	e.dog = restarted

	e.at(t, 30*time.Minute)
	if len(e.posts(t, "#dms-whatsapp")) != 1 {
		t.Fatalf("a restarted watchdog posted again within the hour")
	}
}

func TestExitedAgent(t *testing.T) {
	e := newEnv(t)

	code := 2
	finished := start.Add(time.Minute)
	e.probe.job.Running = false
	e.probe.job.ExitCode = &code
	e.probe.job.FinishedAt = &finished
	e.probe.job.LastLine = "unknown flag: --brief"

	e.at(t, 13*time.Minute)
	want := "dms-whatsapp's agent exited with code 2 12 minutes ago and the task is not done. Last terminal line: `unknown flag: --brief`"
	got := e.posts(t, "#dms-whatsapp")
	if len(got) != 1 || got[0] != want {
		t.Fatalf("#dms-whatsapp = %q, want [%q]", got, want)
	}
}

func TestReviewWaitsTooLong(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()

	delivered := start.Add(10 * time.Minute)
	e.task.DeliveredAt = &delivered
	_, err := e.db.NewUpdate().Model(e.task).Column("delivered_at").WherePK().Exec(ctx)
	if err != nil {
		t.Fatal(err)
	}

	// In review the agent waits by design: no silence report.
	e.at(t, time.Hour)
	if len(e.posts(t, "#dms-whatsapp")) != 0 {
		t.Fatalf("reported silence while in review")
	}

	e.at(t, 2*time.Hour+15*time.Minute)
	want := "dms-whatsapp has been in review for 2 hours 5 minutes, waiting on @orchestrator or @franco to /accept or /reject it."
	got := e.posts(t, "#franco")
	if len(got) != 1 || got[0] != want {
		t.Fatalf("#franco = %q, want [%q]", got, want)
	}
}

func TestDoneAndStoppedTasksAreLeftAlone(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()

	e.probe.running = false
	e.at(t, time.Hour)
	if len(e.posts(t, "#dms-whatsapp")) != 0 {
		t.Fatalf("reported a stopped sandbox")
	}

	e.probe.running = true
	done := start
	e.task.DoneAt = &done
	_, err := e.db.NewUpdate().Model(e.task).Column("done_at").WherePK().Exec(ctx)
	if err != nil {
		t.Fatal(err)
	}

	e.at(t, 2*time.Hour)
	if len(e.posts(t, "#dms-whatsapp")) != 0 {
		t.Fatalf("reported a done task")
	}
}

func TestAgo(t *testing.T) {
	cases := map[time.Duration]string{
		time.Minute:                  "1 minute",
		52 * time.Minute:             "52 minutes",
		2 * time.Hour:                "2 hours",
		2*time.Hour + time.Minute:    "2 hours 1 minute",
		26*time.Hour + 5*time.Minute: "26 hours 5 minutes",
	}
	for d, want := range cases {
		got := Ago(d)
		if got != want {
			t.Errorf("Ago(%v) = %q, want %q", d, got, want)
		}
	}
}
