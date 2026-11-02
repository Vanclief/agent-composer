package schedules

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"github.com/vanclief/ez"

	"github.com/vanclief/agent-composer/core/events"
	"github.com/vanclief/agent-composer/core/resources/tasks"
	schedulemodels "github.com/vanclief/agent-composer/models/schedule"
	"github.com/vanclief/agent-composer/secrets"
)

const (
	// keyField is the item field that identifies it across ticks.
	keyField = "key"
	// maxOutputShown caps how much of a failing command's stderr goes in
	// last_error.
	maxOutputShown = 2000
	defaultPath    = "/usr/local/bin:/usr/bin:/bin"
)

// claim marks the schedule as ticking, false when it already is.
func (api *API) claim(id uuid.UUID) bool {
	api.mu.Lock()
	defer api.mu.Unlock()

	if api.running[id] {
		return false
	}

	api.running[id] = true
	api.ticks.Add(1)

	return true
}

func (api *API) release(id uuid.UUID) {
	api.mu.Lock()
	delete(api.running, id)
	api.mu.Unlock()

	api.ticks.Done()
}

// tick runs the schedule once. Unless force is set it runs only when the
// schedule is still enabled and due once claimed, and returns nil when it
// is not. A schedule already ticking is a conflict: ticks never overlap.
func (api *API) tick(ctx context.Context, id uuid.UUID, name string, force bool) (*RunResponse, error) {
	if !api.claim(id) {
		return nil, ez.New(ez.ECONFLICT, "schedule "+name+" is already running: wait for its tick to end", nil)
	}

	response, err := api.claimedTick(ctx, id, force)
	api.release(id)

	if response != nil {
		response.Schedule.Running = false
	}

	return response, err
}

func (api *API) claimedTick(ctx context.Context, id uuid.UUID, force bool) (*RunResponse, error) {
	record, err := schedulemodels.GetByID(ctx, api.db, id)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	began := time.Now()
	at := api.now()
	if !force && (!record.Enabled || record.NextRunAt().After(at)) {
		return nil, nil
	}

	created, problems := api.runOnce(ctx, record, at)
	lastError := strings.Join(problems, "\n")

	err = record.MarkRun(context.WithoutCancel(ctx), api.db, at, lastError)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	names := make([]string, 0, len(created))
	for _, view := range created {
		names = append(names, view.Name)
	}

	event := log.Info()
	if lastError != "" {
		event = log.Warn().Str("error", lastError)
	}
	event.Str("schedule", record.Name).Bool("forced", force).Strs("created", names).
		Dur("took", time.Since(began)).Msg("schedule ran")

	view := *api.view(record)
	view.Running = false
	api.events.Publish(events.Event{Type: events.ScheduleRan, Data: RanEvent{Schedule: view, Created: created}})

	return &RunResponse{Schedule: view, Created: created, Error: lastError}, nil
}

// runOnce creates the tick's tasks and returns them with what went wrong.
// A poll that fails creates nothing; a task that cannot be created leaves
// its item unrecorded, so the next tick tries it again.
func (api *API) runOnce(ctx context.Context, record *schedulemodels.Schedule, at time.Time) ([]tasks.TaskView, []string) {
	created := []tasks.TaskView{}

	if record.Poll == nil {
		view, err := api.createTask(ctx, record, itemData(nil, at), defaultName(record.Name, timeSuffix(at)))
		if err != nil {
			return created, []string{ez.ErrorMessage(err)}
		}

		err = api.record(ctx, record, view.Name, view)
		if err != nil {
			return []tasks.TaskView{*view}, []string{ez.ErrorMessage(err)}
		}

		return []tasks.TaskView{*view}, nil
	}

	items, err := api.poll(ctx, record.Poll)
	if err != nil {
		return created, []string{ez.ErrorMessage(err)}
	}

	keys := make([]string, 0, len(items))
	for _, item := range items {
		keys = append(keys, item[keyField])
	}

	seen, err := schedulemodels.SeenKeys(ctx, api.db, record.ID, keys)
	if err != nil {
		return created, []string{ez.ErrorMessage(err)}
	}

	problems := []string{}
	for _, item := range items {
		key := item[keyField]
		if seen[key] {
			continue
		}
		seen[key] = true

		view, err := api.createTask(ctx, record, itemData(item, at), defaultName(record.Name, keySuffix(key)))
		if err != nil {
			problems = append(problems, fmt.Sprintf("item %s: %s", key, ez.ErrorMessage(err)))
			continue
		}
		created = append(created, *view)

		err = api.record(ctx, record, key, view)
		if err != nil {
			problems = append(problems, fmt.Sprintf("item %s: task %s created but not recorded: %s", key, view.Name, ez.ErrorMessage(err)))
		}
	}

	return created, problems
}

// createTask renders the template over data and creates the task exactly
// as POST /api/tasks does: defaults, opening message, allocation.
func (api *API) createTask(ctx context.Context, record *schedulemodels.Schedule, data map[string]any, fallbackName string) (*tasks.TaskView, error) {
	rendered, err := renderTemplate(record.Task, data)
	if err != nil {
		return nil, err
	}

	return api.tasks.Create(ctx, nil, createRequest(rendered, fallbackName))
}

func (api *API) record(ctx context.Context, record *schedulemodels.Schedule, key string, view *tasks.TaskView) error {
	item := &schedulemodels.Item{ScheduleID: record.ID, Key: key, TaskID: view.ID, Task: view.Name}

	return item.Insert(context.WithoutCancel(ctx), api.db)
}

// poll runs the command and reads its stdout as JSON lines.
func (api *API) poll(ctx context.Context, poll *schedulemodels.Poll) ([]map[string]string, error) {
	env, values, err := api.pollEnv(poll.Secrets)
	if err != nil {
		return nil, err
	}

	runCtx, cancel := context.WithTimeout(ctx, api.pollTimeout)
	defer cancel()

	stdout, stderr, err := api.run(runCtx, poll.Command, env)
	if errors.Is(runCtx.Err(), context.DeadlineExceeded) {
		return nil, ez.New(ez.EUNAVAILABLE, fmt.Sprintf("poll command timed out after %s", api.pollTimeout), err)
	}
	if err != nil {
		msg := "poll command failed: " + err.Error()
		shown := strings.TrimSpace(string(stderr))
		if shown != "" {
			msg += ": " + tail(shown)
		}
		return nil, ez.New(ez.EUNAVAILABLE, redact(msg, values), nil)
	}

	items, err := parseItems(stdout)
	if err != nil {
		return nil, ez.New(ez.EINVALID, redact(ez.ErrorMessage(err), values), nil)
	}

	return items, nil
}

// pollEnv is the command's whole environment: PATH, HOME and the named
// secrets from the secrets file, plus the values to keep out of messages.
func (api *API) pollEnv(names []string) ([]string, map[string]string, error) {
	path := os.Getenv("PATH")
	if path == "" {
		path = defaultPath
	}

	home, err := os.UserHomeDir()
	if err != nil {
		home = "/"
	}

	env := []string{"PATH=" + path, "HOME=" + home}
	values := map[string]string{}
	if len(names) == 0 {
		return env, values, nil
	}

	if api.secretsPath == "" {
		return nil, nil, ez.New(ez.EINVALID, "poll.secrets are set but agc has no secrets file", nil)
	}

	store, err := secrets.Load(api.secretsPath)
	if err != nil {
		return nil, nil, ez.Wrap(err)
	}

	for _, name := range names {
		value, ok := store.Lookup(name)
		if !ok {
			return nil, nil, ez.New(ez.ENOTFOUND, fmt.Sprintf("secret %s is not set in %s", name, api.secretsPath), nil)
		}
		env = append(env, name+"="+value)
		values[name] = value
	}

	return env, values, nil
}

// parseItems reads one JSON object per non-empty line, every value a
// string and key not empty.
func parseItems(stdout []byte) ([]map[string]string, error) {
	items := []map[string]string{}

	scanner := bufio.NewScanner(bytes.NewReader(stdout))
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		raw := map[string]any{}
		err := json.Unmarshal([]byte(line), &raw)
		if err != nil {
			return nil, ez.New(ez.EINVALID, fmt.Sprintf("poll output line %d is not a JSON object: %s", lineNo, tail(line)), err)
		}

		item := map[string]string{}
		for name, value := range raw {
			text, ok := value.(string)
			if !ok {
				return nil, ez.New(ez.EINVALID, fmt.Sprintf("poll output line %d: field %s is not a string", lineNo, name), nil)
			}
			item[name] = text
		}

		if strings.TrimSpace(item[keyField]) == "" {
			return nil, ez.New(ez.EINVALID, fmt.Sprintf("poll output line %d has no key: every line needs a non-empty \"key\"", lineNo), nil)
		}

		items = append(items, item)
	}

	err := scanner.Err()
	if err != nil {
		return nil, ez.New(ez.EINVALID, "cannot read the poll output: "+err.Error(), err)
	}

	return items, nil
}

// runShell runs command with sh -c and env as its whole environment.
func runShell(ctx context.Context, command string, env []string) ([]byte, []byte, error) {
	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	cmd.Env = env
	// A timeout kills the command's children too, and one still holding
	// the pipes does not hang the tick.
	killGroup(cmd)
	cmd.WaitDelay = 5 * time.Second

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	return stdout.Bytes(), stderr.Bytes(), err
}

// redact replaces every secret value in text with its name.
func redact(text string, values map[string]string) string {
	for name, value := range values {
		if value != "" {
			text = strings.ReplaceAll(text, value, "["+name+"]")
		}
	}

	return text
}

// tail keeps the last maxOutputShown bytes of text.
func tail(text string) string {
	if len(text) <= maxOutputShown {
		return text
	}

	return "…" + text[len(text)-maxOutputShown:]
}
