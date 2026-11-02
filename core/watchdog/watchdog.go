// Package watchdog watches the sandboxes of every task that is not done and
// tells the task's channel and its owner, as agc, when the agent has gone
// silent, has exited, or the task has waited too long in review, and
// restarts a sandbox msb reports crashed, a few times an hour at most.
// Every signal is mechanical: no model is asked (ADR 0013, ADR 0017).
package watchdog

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/uptrace/bun"
	"github.com/vanclief/ez"

	"github.com/vanclief/agent-composer/core/controller"
	"github.com/vanclief/agent-composer/core/events"
	"github.com/vanclief/agent-composer/core/resources/sandboxes"
	"github.com/vanclief/agent-composer/messaging"
	msgmodels "github.com/vanclief/agent-composer/models/messaging"
	taskmodels "github.com/vanclief/agent-composer/models/task"
)

const (
	// Interval is how often Run checks.
	Interval = time.Minute
	// Repeat is the least time between two posts about one condition.
	Repeat = time.Hour
	// lookBack is how many of agc's posts in #<task> lastPosted reads.
	lookBack = 50

	// EventAlert is the server-sent event a post or a clear publishes.
	EventAlert = "watchdog.alert"

	KindSilent = "silent"
	KindExited = "exited"
	KindReview = "review"

	// restartWindow is the period Settings.Restarts counts restarts in.
	restartWindow = time.Hour
)

// Poster posts as agc (messaging.Service).
type Poster interface {
	PostAsSystem(ctx context.Context, channel, body string) (*msgmodels.Message, error)
}

// Probe reads a sandbox from the runtime and restarts it (sandboxes.API).
type Probe interface {
	Status(ctx context.Context, name string) (string, error)
	AgentJob(ctx context.Context, record *taskmodels.Sandbox) (*sandboxes.AgentJob, error)
	Restart(ctx context.Context, record *taskmodels.Sandbox, owner *taskmodels.Task) error
}

// Settings are the thresholds; zero turns a check off. Restarts is how
// many times within an hour a crashed sandbox is restarted (ADR 0017).
type Settings struct {
	Silence  time.Duration
	Review   time.Duration
	Restarts int
}

// SettingsFrom is the watchdog's settings from the resolved config.
func SettingsFrom(dog controller.Watchdog, recovery controller.Recovery) Settings {
	return Settings{Silence: dog.Silence, Review: dog.Review, Restarts: recovery.Restarts}
}

// Alert is one condition that holds now.
type Alert struct {
	Kind    string `json:"kind"`
	Task    string `json:"task"`
	Sandbox string `json:"sandbox,omitempty"`
	// Since is when the condition began: the last activity, the exit, or
	// the delivery.
	Since time.Time `json:"since"`
	Body  string    `json:"body"`
	// prefix starts Body and every earlier post about the same condition.
	prefix string
}

func (a Alert) key() string {
	return a.Task + "/" + a.Sandbox + "/" + a.Kind
}

// AlertEvent is the data of watchdog.alert.
type AlertEvent struct {
	Alert
	// Cleared is true when the condition no longer holds; Body is then
	// the last one posted.
	Cleared bool `json:"cleared"`
}

// tracked is what the watchdog remembers about one condition.
type tracked struct {
	alert  Alert
	posted time.Time
	// holds is false once the condition cleared; the entry is kept so a
	// recurrence posts at once instead of looking back in the channel.
	holds bool
}

type Watchdog struct {
	db       bun.IDB
	msg      Poster
	probe    Probe
	events   *events.Bus
	settings Settings
	// now is time.Now, replaced in tests.
	now func() time.Time

	mu    sync.Mutex
	state map[string]*tracked
	// restarts is what the watchdog restarted, by sandbox name.
	restarts map[string]*restartLog
}

// restartLog is one sandbox's recent restarts.
type restartLog struct {
	// at holds when each restart within restartWindow was tried.
	at []time.Time
	// lastErr is the last restart's failure, empty if it started.
	lastErr string
	// gaveUp is set once the limit was reached and said; it clears when
	// the sandbox is seen running again.
	gaveUp bool
}

// New returns a watchdog. bus may be nil.
func New(db bun.IDB, msg Poster, probe Probe, bus *events.Bus, settings Settings) *Watchdog {
	return &Watchdog{
		db:       db,
		msg:      msg,
		probe:    probe,
		events:   bus,
		settings: settings,
		now:      time.Now,
		state:    map[string]*tracked{},
		restarts: map[string]*restartLog{},
	}
}

// Run checks every Interval until ctx ends.
func (w *Watchdog) Run(ctx context.Context) {
	ticker := time.NewTicker(Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			err := w.Check(ctx)
			if err != nil {
				log.Warn().Err(err).Msg("watchdog check failed")
			}
		}
	}
}

// Check looks at every task that is not done once, posts what holds and
// was not posted within Repeat, and forgets what cleared.
func (w *Watchdog) Check(ctx context.Context) error {
	records, err := taskmodels.ListTasks(ctx, w.db)
	if err != nil {
		return ez.Wrap(err)
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	holding := map[string]bool{}
	for i := range records {
		record := &records[i]
		if record.DoneAt != nil {
			continue
		}

		alerts := w.taskAlerts(ctx, record)
		for _, alert := range alerts {
			holding[alert.key()] = true
			w.raise(ctx, record, alert)
		}
	}

	for key, entry := range w.state {
		if holding[key] || !entry.holds {
			continue
		}
		entry.holds = false
		w.events.Publish(events.Event{Type: EventAlert, Data: AlertEvent{Alert: entry.alert, Cleared: true}})
	}

	return nil
}

// raise posts alert unless it was posted within Repeat. After a restart
// the channel is the memory: agc's own earlier post with the same prefix.
func (w *Watchdog) raise(ctx context.Context, record *taskmodels.Task, alert Alert) {
	now := w.now()
	key := alert.key()

	entry, known := w.state[key]
	if !known {
		entry = &tracked{alert: alert}
		w.state[key] = entry
		entry.posted = w.lastPosted(ctx, record.Name, alert.prefix, now)
	}
	if known && !entry.holds {
		entry.posted = time.Time{}
	}
	entry.holds = true
	entry.alert = alert

	if !entry.posted.IsZero() && now.Sub(entry.posted) < Repeat {
		return
	}

	w.tell(ctx, record, alert.Body)

	entry.posted = now
	w.events.Publish(events.Event{Type: EventAlert, Data: AlertEvent{Alert: alert}})
}

// lastPosted is when agc last posted a message starting with prefix in
// #<task> within Repeat, zero if it did not.
func (w *Watchdog) lastPosted(ctx context.Context, task string, prefix string, now time.Time) time.Time {
	messages := []msgmodels.Message{}

	err := w.db.NewSelect().Model(&messages).
		Where("channel = ?", messaging.TaskChannel(task)).
		Where(`"from" = ?`, msgmodels.SystemName).
		Order("id DESC").
		Limit(lookBack).
		Scan(ctx)
	if err != nil {
		log.Warn().Err(err).Str("task", task).Msg("watchdog cannot read its earlier posts")
		return time.Time{}
	}

	for _, message := range messages {
		if now.Sub(message.CreatedAt) >= Repeat {
			break
		}
		if strings.HasPrefix(message.Body, prefix) {
			return message.CreatedAt
		}
	}

	return time.Time{}
}

// taskAlerts are the conditions holding for one task that is not done.
func (w *Watchdog) taskAlerts(ctx context.Context, record *taskmodels.Task) []Alert {
	now := w.now()
	alerts := []Alert{}

	inReview := record.DeliveredAt != nil
	if inReview && w.settings.Review > 0 && now.Sub(*record.DeliveredAt) >= w.settings.Review {
		alerts = append(alerts, reviewAlert(record, now))
	}

	boxes, err := taskmodels.ListTaskSandboxes(ctx, w.db, record.ID)
	if err != nil {
		log.Warn().Err(err).Str("task", record.Name).Msg("watchdog cannot list sandboxes")
		return alerts
	}

	for i := range boxes {
		box := &boxes[i]
		if box.BootedAt == nil || box.BootError != nil {
			continue
		}

		status, err := w.probe.Status(ctx, box.Name)
		if err != nil {
			continue
		}

		if status == sandboxes.StatusCrashed && sandboxes.Recoverable(box, record, status) {
			w.restart(ctx, record, box)
			continue
		}

		if status != sandboxes.StatusRunning {
			continue
		}

		w.seenRunning(box.Name)

		if box.Command == "" {
			continue
		}

		job, err := w.probe.AgentJob(ctx, box)
		if err != nil {
			log.Warn().Err(err).Str("sandbox", box.Name).Msg("watchdog cannot read the agent job")
			continue
		}
		if job == nil {
			continue
		}

		if !job.Running {
			alerts = append(alerts, exitedAlert(record, box, job, now))
			continue
		}

		if inReview || w.settings.Silence <= 0 {
			continue
		}

		alert, silent := w.silentAlert(ctx, record, box, job, now)
		if silent {
			alerts = append(alerts, alert)
		}
	}

	return alerts
}

// restart starts a crashed sandbox again, at most Settings.Restarts times
// within restartWindow, and says each try in #<task> and #<owner>. Past
// the limit it gives up and says so once, with the reason, until the
// sandbox is seen running again. Callers hold w.mu.
func (w *Watchdog) restart(ctx context.Context, record *taskmodels.Task, box *taskmodels.Sandbox) {
	if w.settings.Restarts <= 0 {
		return
	}

	entry, ok := w.restarts[box.Name]
	if !ok {
		entry = &restartLog{}
		w.restarts[box.Name] = entry
	}
	if entry.gaveUp {
		return
	}

	now := w.now()
	recent := []time.Time{}
	for _, at := range entry.at {
		if now.Sub(at) < restartWindow {
			recent = append(recent, at)
		}
	}
	entry.at = recent

	if len(entry.at) >= w.settings.Restarts {
		entry.gaveUp = true
		w.tell(ctx, record, GaveUp(box.Name, len(entry.at), entry.lastErr))
		return
	}

	entry.at = append(entry.at, now)
	tries := fmt.Sprintf("restart %d of %d this hour", len(entry.at), w.settings.Restarts)

	log.Info().Str("task", record.Name).Str("sandbox", box.Name).Msg("restarting a crashed sandbox")

	err := w.probe.Restart(ctx, box, record)
	reason, ok := sandboxes.NotRelaunched(err)
	if ok {
		entry.lastErr = ""
		w.tell(ctx, record, fmt.Sprintf("%s crashed; agc restarted it (%s), but %s.", box.Name, tries, reason))
		return
	}
	if err != nil {
		entry.lastErr = ez.ErrorMessage(err)
		log.Warn().Err(err).Str("sandbox", box.Name).Msg("watchdog cannot restart the crashed sandbox")
		w.tell(ctx, record, fmt.Sprintf("%s crashed and agc could not restart it (%s): %s", box.Name, tries, entry.lastErr))
		return
	}

	entry.lastErr = ""
	continues := "its agent continues."
	if box.Command == "" {
		continues = "it is running again."
	}
	w.tell(ctx, record, fmt.Sprintf("%s crashed; agc restarted it (%s) and %s", box.Name, tries, continues))
}

// GaveUp is the one line the watchdog posts when it stops restarting a
// sandbox.
func GaveUp(sandbox string, restarts int, lastErr string) string {
	body := fmt.Sprintf("agc gave up restarting %s: it crashed again after %s within an hour", sandbox, plural(restarts, "restart"))
	if lastErr != "" {
		body = fmt.Sprintf("agc gave up restarting %s: %s within an hour, the last failed with: %s", sandbox, plural(restarts, "restart"), lastErr)
	}

	return body + ". It stays down until someone starts it."
}

// seenRunning lets the watchdog restart a sandbox again after it gave up,
// once someone started it. Callers hold w.mu.
func (w *Watchdog) seenRunning(name string) {
	entry, ok := w.restarts[name]
	if ok {
		entry.gaveUp = false
	}
}

// tell posts body as agc in #<task> and, when the task has one, #<owner>.
func (w *Watchdog) tell(ctx context.Context, record *taskmodels.Task, body string) {
	channels := []string{messaging.TaskChannel(record.Name)}
	if record.Owner != "" {
		channels = append(channels, "#"+record.Owner)
	}

	for _, channel := range channels {
		_, err := w.msg.PostAsSystem(ctx, channel, body)
		if err != nil {
			log.Warn().Err(err).Str("task", record.Name).Str("channel", channel).Msg("watchdog cannot post")
		}
	}
}

// silentAlert holds when the agent job has written nothing to its terminal
// and nobody but agc has posted in #<task> for Silence, and the last word
// in the channel is not the agent's: an agent that answered and waits for
// its manager is not stuck.
func (w *Watchdog) silentAlert(ctx context.Context, record *taskmodels.Task, box *taskmodels.Sandbox, job *sandboxes.AgentJob, now time.Time) (Alert, bool) {
	last := latest(*box.BootedAt, job.StartedAt, job.LastOutputAt)

	message, err := lastMessage(ctx, w.db, record.Name)
	if err != nil {
		log.Warn().Err(err).Str("task", record.Name).Msg("watchdog cannot read the channel")
		return Alert{}, false
	}
	if message != nil {
		if message.Task == record.Name {
			return Alert{}, false
		}
		last = latest(last, message.CreatedAt)
	}

	if now.Sub(last) < w.settings.Silence {
		return Alert{}, false
	}

	prefix := box.Name + " has been silent for "
	body := prefix + Ago(now.Sub(last)) + ". " + lastLine(job.LastLine)

	return Alert{Kind: KindSilent, Task: record.Name, Sandbox: box.Name, Since: last, Body: body, prefix: prefix}, true
}

func exitedAlert(record *taskmodels.Task, box *taskmodels.Sandbox, job *sandboxes.AgentJob, now time.Time) Alert {
	since := job.StartedAt
	if job.FinishedAt != nil {
		since = *job.FinishedAt
	}

	code := "an unknown code"
	if job.ExitCode != nil {
		code = fmt.Sprintf("code %d", *job.ExitCode)
	}

	prefix := box.Name + "'s agent exited"
	body := fmt.Sprintf("%s with %s %s ago and the task is not done. %s", prefix, code, Ago(now.Sub(since)), lastLine(job.LastLine))

	return Alert{Kind: KindExited, Task: record.Name, Sandbox: box.Name, Since: since, Body: body, prefix: prefix}
}

func reviewAlert(record *taskmodels.Task, now time.Time) Alert {
	who := []string{}
	if record.Manager != "" {
		who = append(who, "@"+record.Manager)
	}
	if record.Owner != "" && record.Owner != record.Manager {
		who = append(who, "@"+record.Owner)
	}
	waiting := "anyone"
	if len(who) > 0 {
		waiting = strings.Join(who, " or ")
	}

	prefix := record.Name + " has been in review for "
	body := fmt.Sprintf("%s%s, waiting on %s to /accept or /reject it.", prefix, Ago(now.Sub(*record.DeliveredAt)), waiting)

	return Alert{Kind: KindReview, Task: record.Name, Since: *record.DeliveredAt, Body: body, prefix: prefix}
}

// lastMessage is the newest message in #<task> not posted by agc, nil if
// there is none. agc's own posts (the opening, deliveries, the watchdog)
// are not activity.
func lastMessage(ctx context.Context, db bun.IDB, task string) (*msgmodels.Message, error) {
	messages := []msgmodels.Message{}

	err := db.NewSelect().Model(&messages).
		Where("channel = ?", messaging.TaskChannel(task)).
		Where(`"from" <> ?`, msgmodels.SystemName).
		Order("id DESC").
		Limit(1).
		Scan(ctx)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	if len(messages) == 0 {
		return nil, nil
	}

	return &messages[0], nil
}

func lastLine(line string) string {
	if line == "" {
		return "Its terminal is empty."
	}

	return "Last terminal line: `" + strings.ReplaceAll(line, "`", "'") + "`"
}

func latest(times ...time.Time) time.Time {
	var max time.Time
	for _, t := range times {
		if t.After(max) {
			max = t
		}
	}

	return max
}

// Ago says a duration in minutes, or hours and minutes from two hours on:
// "1 minute", "52 minutes", "2 hours 5 minutes".
func Ago(d time.Duration) string {
	minutes := int(d / time.Minute)
	if minutes < 120 {
		return plural(minutes, "minute")
	}

	hours := minutes / 60
	rest := minutes % 60
	if rest == 0 {
		return plural(hours, "hour")
	}

	return plural(hours, "hour") + " " + plural(rest, "minute")
}

func plural(n int, unit string) string {
	if n == 1 {
		return "1 " + unit
	}

	return fmt.Sprintf("%d %ss", n, unit)
}
