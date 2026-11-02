// Package triggers turns events from outside agc into messages in agc
// channels (docs/decisions/0014-triggers.md). The one source is GitHub,
// polled: agc's services bind to 127.0.0.1, so nothing can call in.
// Routing is code: a pull request whose head branch is agc/<task> goes to
// #<task> when that task exists, everything else to #main.
package triggers

import (
	"context"
	"net/http"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/uptrace/bun"
	"github.com/vanclief/ez"

	"github.com/vanclief/agent-composer/core/controller"
	"github.com/vanclief/agent-composer/messaging"
	msgmodels "github.com/vanclief/agent-composer/models/messaging"
	taskmodels "github.com/vanclief/agent-composer/models/task"
	"github.com/vanclief/agent-composer/secrets"
)

const (
	// MainChannel is the main agent's channel, where every event that
	// belongs to no task goes.
	MainChannel = "#main"
	// TaskBranchPrefix marks a head branch as a task's: agc/<task>.
	TaskBranchPrefix = "agc/"
	// Overlap is how far before its cursor a poll looks again, so an
	// event GitHub lists late is still posted; keys seen inside it are
	// skipped.
	Overlap = 5 * time.Minute
	// MaxText is how many characters of an event's text a message keeps.
	MaxText = 400
	// TokenSecret is the secret the GitHub source reads from the secrets
	// file (ADR 0004).
	TokenSecret = "GITHUB_TOKEN"
	// pollTimeout bounds one poll of every repo.
	pollTimeout = 2 * time.Minute
)

// Poster posts a message as agc; messaging.Service is one.
type Poster interface {
	PostAsSystem(ctx context.Context, channel, body string) (*msgmodels.Message, error)
}

// TaskExists reports whether a task by that name exists, done or not.
type TaskExists func(ctx context.Context, name string) (bool, error)

// Event is one thing that happened at a source, ready to post.
type Event struct {
	// Key identifies the event within its source, so it posts once.
	Key string
	// At is when it happened; the window filters on it.
	At   time.Time
	Repo string
	// Actor is the login that did it, empty when the source does not say.
	Actor string
	// What is the verb and the place: "commented on PR #12 (agc/x)".
	What string
	Text string
	URL  string
	// Branch is the pull request's head branch, or the run's, for routing.
	Branch string
}

// Body is the message: "[github] franco commented on PR #12 (agc/x) in
// owner/repo: text...", then the link on its own line.
func (e Event) Body() string {
	actor := e.Actor
	if actor == "" {
		actor = "someone"
	}

	body := "[github] " + actor + " " + e.What + " in " + e.Repo
	text := Cut(e.Text, MaxText)
	if text != "" {
		body += ": " + text
	}

	if e.URL != "" {
		body += "\n" + e.URL
	}

	return body
}

// Cut folds whitespace to single spaces and keeps at most max characters,
// ending with "..." when it cut.
func Cut(text string, max int) string {
	folded := strings.Join(strings.Fields(text), " ")
	runes := []rune(folded)
	if len(runes) <= max {
		return folded
	}

	return strings.TrimRight(string(runes[:max]), " ") + "..."
}

// API polls the configured sources and posts what is new.
type API struct {
	github      controller.GitHubTriggers
	poster      Poster
	taskExists  TaskExists
	secretsPath string
	store       *Store

	// base, client and now are replaced in tests.
	base   string
	client *http.Client
	now    func() time.Time
}

// StatePath is the cursor file under the agc home.
func StatePath(home string) string {
	return filepath.Join(home, "triggers", "cursors.json")
}

// NewAPI returns the triggers API over the controller's config and
// database, posting through msg, with cursors under home.
func NewAPI(ctrl *controller.Controller, msg *messaging.Service, secretsPath, home string) *API {
	if ctrl == nil {
		panic("Controller reference is nil")
	}

	return New(ctrl.Config.Triggers.GitHub, msg, DBTaskExists(ctrl.DB), secretsPath, StatePath(home))
}

// New is NewAPI over bare parts, for tests.
func New(github controller.GitHubTriggers, poster Poster, taskExists TaskExists, secretsPath, statePath string) *API {
	return &API{
		github:      github,
		poster:      poster,
		taskExists:  taskExists,
		secretsPath: secretsPath,
		store:       NewStore(statePath),
		base:        DefaultGitHubAPI,
		client:      &http.Client{Timeout: 30 * time.Second},
		now:         time.Now,
	}
}

// DBTaskExists looks tasks up by name in db.
func DBTaskExists(db bun.IDB) TaskExists {
	return func(ctx context.Context, name string) (bool, error) {
		_, err := taskmodels.GetTaskByName(ctx, db, name)
		if ez.ErrorCode(err) == ez.ENOTFOUND {
			return false, nil
		}
		if err != nil {
			return false, err
		}

		return true, nil
	}
}

// Route picks the channel for an event: #<task> when its branch is
// agc/<task> and that task exists, else #main.
func (api *API) Route(ctx context.Context, event Event) string {
	name, found := strings.CutPrefix(event.Branch, TaskBranchPrefix)
	if !found || name == "" {
		return MainChannel
	}

	channel := messaging.TaskChannel(name)
	err := msgmodels.ValidateChannel(channel)
	if err != nil {
		return MainChannel
	}

	exists, err := api.taskExists(ctx, name)
	if err != nil {
		log.Warn().Err(err).Str("task", name).Msg("triggers: cannot look the task up, posting in " + MainChannel)
		return MainChannel
	}

	if !exists {
		return MainChannel
	}

	return channel
}

// Start polls every configured source until ctx is canceled: at once and
// then every triggers.github.poll. With no repos it logs that triggers
// are off and returns. Only agc rest starts it.
func (api *API) Start(ctx context.Context) {
	if !api.github.Enabled() {
		log.Info().Msg("triggers: no triggers.github.repos configured, triggers are off")
		return
	}

	every, err := api.github.PollInterval()
	if err == nil {
		err = api.github.Validate()
	}
	if err != nil {
		log.Error().Err(err).Msg("triggers: bad triggers.github config, triggers are off")
		return
	}

	log.Info().Dur("every", every).Strs("repos", api.github.RepoList()).Msg("triggers: github polling started")

	ticker := time.NewTicker(every)
	defer ticker.Stop()

	for {
		api.PollOnce(ctx)

		select {
		case <-ctx.Done():
			log.Info().Msg("triggers: github polling stopped")
			return
		case <-ticker.C:
		}
	}
}

// SourceName is a cursor's key: github <owner/repo> <kind>.
func SourceName(repo, kind string) string {
	return "github " + repo + " " + kind
}

// PollOnce polls every repo and kind once and posts what is new. A kind
// that fails keeps its cursor and records the error; the others go on.
func (api *API) PollOnce(ctx context.Context) {
	ctx, cancel := context.WithTimeout(ctx, pollTimeout)
	defer cancel()

	token, tokenErr := api.token()

	for _, repo := range api.github.RepoList() {
		poll := &repoPoll{
			client: &githubClient{base: api.base, token: token, http: api.client},
			repo:   repo,
			pulls:  map[int]*ghPull{},
		}

		for _, kind := range GitHubKinds {
			if ctx.Err() != nil {
				return
			}

			source := SourceName(repo, kind)
			err := tokenErr
			if err == nil {
				err = api.pollKind(ctx, poll, source, kind)
			}

			if err != nil {
				log.Warn().Err(err).Str("source", source).Msg("triggers: poll failed")
				api.recordError(source, err, token)
			}
		}
	}
}

// token reads GITHUB_TOKEN from the secrets file on every poll, through
// Resolve so a missing secret says where it was looked for.
func (api *API) token() (string, error) {
	store, err := secrets.Load(api.secretsPath)
	if err != nil {
		return "", err
	}

	bindings, err := store.Resolve([]secrets.Request{{Name: TokenSecret}})
	if err != nil {
		return "", err
	}

	return bindings[0].Value, nil
}

func (api *API) recordError(source string, err error, token string) {
	msg := err.Error()
	if token != "" {
		msg = strings.ReplaceAll(msg, token, "[redacted]")
	}

	polled := api.now()
	saveErr := api.store.Update(source, func(cursor *Cursor) {
		cursor.LastPolled = &polled
		cursor.LastError = msg
	})
	if saveErr != nil {
		log.Warn().Err(saveErr).Str("source", source).Msg("triggers: cannot save the cursor")
	}
}

// pollKind polls one kind of one repo. The first poll of a source only
// sets its cursor to now: history is not posted. Later polls post every
// event inside [cursor - Overlap, now) not yet seen, oldest first, and
// save the cursor after each post, so a crash posts nothing twice.
func (api *API) pollKind(ctx context.Context, poll *repoPoll, source, kind string) error {
	started := api.now()

	cursor, err := api.store.Get(source)
	if err != nil {
		return err
	}

	if cursor == nil || cursor.Cursor.IsZero() {
		return api.store.Update(source, func(c *Cursor) {
			c.Cursor = started
			c.LastPolled = &started
			c.LastError = ""
		})
	}

	poll.since = cursor.Cursor.Add(-Overlap)
	events, err := poll.fetch(ctx, kind)
	if err != nil {
		return err
	}

	sort.SliceStable(events, func(i, j int) bool {
		return events[i].At.Before(events[j].At)
	})

	for _, event := range events {
		_, seen := cursor.Seen[event.Key]
		if seen {
			continue
		}

		posted := !api.ignored(event.Actor)
		if posted {
			_, err = api.poster.PostAsSystem(ctx, api.Route(ctx, event), event.Body())
			if err != nil {
				return err
			}
		}

		key := event.Key
		at := event.At
		err = api.store.Update(source, func(c *Cursor) {
			c.Seen[key] = at
			if posted {
				c.Posted++
			}
		})
		if err != nil {
			return err
		}
		cursor.Seen[key] = at
	}

	return api.store.Update(source, func(c *Cursor) {
		c.Cursor = started
		c.LastPolled = &started
		c.LastError = ""
		floor := started.Add(-Overlap)
		for key, at := range c.Seen {
			if at.Before(floor) {
				delete(c.Seen, key)
			}
		}
	})
}

// ignored reports whether login is in triggers.github.ignore_users, case
// insensitively.
func (api *API) ignored(login string) bool {
	for _, user := range api.github.IgnoreUsers {
		if login != "" && strings.EqualFold(strings.TrimSpace(user), login) {
			return true
		}
	}

	return false
}

// SourceView is one source for agc triggers show.
type SourceView struct {
	Source     string     `json:"source"`
	Configured bool       `json:"configured"`
	Cursor     *time.Time `json:"cursor"`
	LastPolled *time.Time `json:"last_polled"`
	LastError  string     `json:"last_error"`
	Posted     int        `json:"posted"`
}

// Show lists every configured source, then any source with a cursor that
// is no longer configured, with its cursor and last poll.
func (api *API) Show() ([]SourceView, error) {
	cursors, err := api.store.All()
	if err != nil {
		return nil, err
	}

	byName := map[string]Cursor{}
	for _, cursor := range cursors {
		byName[cursor.Source] = cursor
	}

	views := []SourceView{}
	listed := map[string]bool{}
	for _, repo := range api.github.RepoList() {
		for _, kind := range GitHubKinds {
			source := SourceName(repo, kind)
			listed[source] = true
			view := SourceView{Source: source, Configured: true}
			cursor, ok := byName[source]
			if ok {
				view = cursorView(cursor, true)
			}
			views = append(views, view)
		}
	}

	for _, cursor := range cursors {
		if !listed[cursor.Source] {
			views = append(views, cursorView(cursor, false))
		}
	}

	return views, nil
}

func cursorView(cursor Cursor, configured bool) SourceView {
	view := SourceView{
		Source:     cursor.Source,
		Configured: configured,
		LastPolled: cursor.LastPolled,
		LastError:  cursor.LastError,
		Posted:     cursor.Posted,
	}

	if !cursor.Cursor.IsZero() {
		at := cursor.Cursor
		view.Cursor = &at
	}

	return view
}

// GitHub returns the GitHub settings the API polls with.
func (api *API) GitHub() controller.GitHubTriggers {
	return api.github
}

// StorePath is the cursor file's path.
func (api *API) StorePath() string {
	return api.store.Path()
}
